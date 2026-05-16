package trigger

// Email-inbound adapter contract.
//
// IMAP is the v1 transport per design.md §D2 / Open Question Q1. Gmail OAuth
// and Outlook are follow-up adapters. This file defines the contract and
// ships a NoopMailbox for dev mode; production wires an IMAP client (e.g.
// emersion/go-imap) into the IMAPMailbox shape.
//
// Each subscription of type email-inbound MUST carry config:
//   - mailbox_ref: secret/credential reference resolved via SecretResolver
//   - filter (optional): { subject_contains?: string, from_matches?: string }
// Outputs available to steps via $triggers.<id>.<field>:
//   - subject, from, body, received_at, message_id

import (
	"context"
	"log"
	"strings"
	"sync"
	"time"

	"github.com/forge-eng-fabric/pkg/workflow/ast"
)

// EmailMessage is the normalised message shape the dispatcher receives
// from every email adapter.
type EmailMessage struct {
	MessageID  string
	Subject    string
	From       string
	Body       string
	ReceivedAt time.Time
}

// Mailbox abstracts an inbox source. Adapters (IMAP, Gmail OAuth,
// Outlook) implement this interface.
type Mailbox interface {
	// Poll fetches every message matching the per-subscription filter
	// that has not been delivered before. Implementations MUST be
	// idempotent across calls — duplicate delivery is the dispatcher's
	// problem to dedupe (it uses MessageID).
	Poll(ctx context.Context, mailboxRef string, since time.Time) ([]EmailMessage, error)
}

// NoopMailbox returns no messages. Wired by default in dev mode and tests
// that don't exercise the email path.
type NoopMailbox struct{}

func (NoopMailbox) Poll(context.Context, string, time.Time) ([]EmailMessage, error) {
	return nil, nil
}

// FixtureMailbox is a deterministic mailbox for tests. Pre-load it with
// messages; Poll returns them once each.
type FixtureMailbox struct {
	Messages []EmailMessage
	mu       sync.Mutex
	served   map[string]bool
}

func (m *FixtureMailbox) Poll(_ context.Context, _ string, _ time.Time) ([]EmailMessage, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.served == nil {
		m.served = map[string]bool{}
	}
	out := []EmailMessage{}
	for _, msg := range m.Messages {
		if m.served[msg.MessageID] {
			continue
		}
		m.served[msg.MessageID] = true
		out = append(out, msg)
	}
	return out, nil
}

// EmailPoller scans every email-inbound subscription on each Tick. The
// caller schedules ticks (production: every 30s; tests: explicit Tick()).
type EmailPoller struct {
	Registry   *Registry
	Dispatcher *Dispatcher
	Mailbox    Mailbox

	mu       sync.Mutex
	lastSeen map[Key]time.Time
}

// NewEmailPoller constructs a poller. Mailbox is the adapter to use for
// every email-inbound subscription's mailbox_ref.
func NewEmailPoller(reg *Registry, dispatch *Dispatcher, mailbox Mailbox) *EmailPoller {
	return &EmailPoller{
		Registry:   reg,
		Dispatcher: dispatch,
		Mailbox:    mailbox,
		lastSeen:   map[Key]time.Time{},
	}
}

// Tick polls every email-inbound subscription once and dispatches matching
// messages. Returns the count of messages dispatched.
func (p *EmailPoller) Tick(ctx context.Context) int {
	subs := p.Registry.ByType(ast.TriggerEmailInbound)
	dispatched := 0
	for _, sub := range subs {
		k := Key{sub.WorkflowID, sub.TriggerID, sub.Version}
		mailboxRef, _ := sub.Config["mailbox_ref"].(string)
		if mailboxRef == "" {
			continue
		}
		p.mu.Lock()
		since := p.lastSeen[k]
		p.mu.Unlock()
		msgs, err := p.Mailbox.Poll(ctx, mailboxRef, since)
		if err != nil {
			log.Printf("email poll failed for %v: %v", k, err)
			continue
		}
		filter := extractFilter(sub.Config["filter"])
		for _, msg := range msgs {
			if !matchFilter(filter, msg) {
				continue
			}
			_, ferr := p.Dispatcher.Fire(ctx, sub, map[string]any{
				"subject":     msg.Subject,
				"from":        msg.From,
				"body":        msg.Body,
				"received_at": msg.ReceivedAt,
				"message_id":  msg.MessageID,
			})
			if ferr != nil {
				log.Printf("email dispatch failed for %v: %v", k, ferr)
				continue
			}
			dispatched++
			p.mu.Lock()
			if msg.ReceivedAt.After(p.lastSeen[k]) {
				p.lastSeen[k] = msg.ReceivedAt
			}
			p.mu.Unlock()
		}
	}
	return dispatched
}

func extractFilter(raw any) emailFilter {
	m, ok := raw.(map[string]any)
	if !ok {
		return emailFilter{}
	}
	out := emailFilter{}
	if v, _ := m["subject_contains"].(string); v != "" {
		out.SubjectContains = v
	}
	if v, _ := m["from_matches"].(string); v != "" {
		out.FromMatches = v
	}
	return out
}

type emailFilter struct {
	SubjectContains string
	FromMatches     string
}

func matchFilter(f emailFilter, msg EmailMessage) bool {
	if f.SubjectContains != "" && !strings.Contains(msg.Subject, f.SubjectContains) {
		return false
	}
	if f.FromMatches != "" && !strings.Contains(msg.From, f.FromMatches) {
		return false
	}
	return true
}
