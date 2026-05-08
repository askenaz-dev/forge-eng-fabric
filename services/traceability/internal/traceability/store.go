package traceability

import (
	"fmt"
	"sort"
	"sync"
	"time"
)

type MaterializedGraph struct {
	RootOpenSpecID string
	Depth          int
	Nodes          []Node
	Links          []Link
	MaterializedAt time.Time
}

type Store struct {
	mu           sync.RWMutex
	nodes        map[string]*Node
	nodeIndex    map[string]string
	links        map[string]*Link
	linkIndex    map[string]string
	materialized map[string]MaterializedGraph
}

func NewStore() *Store {
	return &Store{
		nodes:        map[string]*Node{},
		nodeIndex:    map[string]string{},
		links:        map[string]*Link{},
		linkIndex:    map[string]string{},
		materialized: map[string]MaterializedGraph{},
	}
}

func (s *Store) UpsertNode(node Node) (Node, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	key := nodeKey(node.WorkspaceID, node.Type, node.ExternalID)
	if id, ok := s.nodeIndex[key]; ok {
		existing := s.nodes[id]
		for k, v := range node.Metadata {
			if existing.Metadata == nil {
				existing.Metadata = map[string]any{}
			}
			existing.Metadata[k] = v
		}
		existing.UpdatedAt = time.Now().UTC()
		return cloneNode(existing), false
	}
	if node.ID == "" {
		node.ID = newID()
	}
	now := time.Now().UTC()
	if node.CreatedAt.IsZero() {
		node.CreatedAt = now
	}
	node.UpdatedAt = now
	s.nodes[node.ID] = cloneNodePtr(&node)
	s.nodeIndex[key] = node.ID
	return node, true
}

func (s *Store) UpsertLink(link Link) (Link, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	key := linkKey(link.FromNodeID, link.ToNodeID, link.Relation, link.SourceEvent)
	if id, ok := s.linkIndex[key]; ok {
		return cloneLink(s.links[id]), false
	}
	if link.ID == "" {
		link.ID = newID()
	}
	if link.CreatedAt.IsZero() {
		link.CreatedAt = time.Now().UTC()
	}
	s.links[link.ID] = cloneLinkPtr(&link)
	s.linkIndex[key] = link.ID
	return link, true
}

func (s *Store) NodeByExternal(workspaceID string, nodeType NodeType, externalID string) (Node, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	id, ok := s.nodeIndex[nodeKey(workspaceID, nodeType, externalID)]
	if !ok {
		return Node{}, false
	}
	return cloneNode(s.nodes[id]), true
}

func (s *Store) Materialized(openSpecID string, depth int) (GraphResponse, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	graph, ok := s.materialized[materializedKey(openSpecID, depth)]
	if !ok {
		return GraphResponse{}, false
	}
	return GraphResponse{OpenSpecID: graph.RootOpenSpecID, Depth: graph.Depth, Nodes: append([]Node(nil), graph.Nodes...), Links: append([]Link(nil), graph.Links...), MaterializedAt: graph.MaterializedAt}, true
}

func (s *Store) RefreshMaterialized(openSpecID string, depth int) GraphResponse {
	s.mu.Lock()
	defer s.mu.Unlock()
	rootIDs := []string{}
	for _, node := range s.nodes {
		if node.Type == NodeOpenSpec && node.ExternalID == openSpecID {
			rootIDs = append(rootIDs, node.ID)
		}
	}
	visited := map[string]int{}
	queue := []string{}
	for _, id := range rootIDs {
		visited[id] = 0
		queue = append(queue, id)
	}
	for len(queue) > 0 {
		current := queue[0]
		queue = queue[1:]
		if visited[current] >= depth {
			continue
		}
		for _, link := range s.links {
			neighbors := []string{}
			if link.FromNodeID == current {
				neighbors = append(neighbors, link.ToNodeID)
			}
			if link.ToNodeID == current {
				neighbors = append(neighbors, link.FromNodeID)
			}
			for _, next := range neighbors {
				if _, seen := visited[next]; seen {
					continue
				}
				visited[next] = visited[current] + 1
				queue = append(queue, next)
			}
		}
	}
	nodes := []Node{}
	for id := range visited {
		if node, ok := s.nodes[id]; ok {
			nodes = append(nodes, cloneNode(node))
		}
	}
	links := []Link{}
	for _, link := range s.links {
		_, fromOK := visited[link.FromNodeID]
		_, toOK := visited[link.ToNodeID]
		if fromOK && toOK {
			links = append(links, cloneLink(link))
		}
	}
	sort.Slice(nodes, func(i, j int) bool { return nodes[i].ID < nodes[j].ID })
	sort.Slice(links, func(i, j int) bool { return links[i].ID < links[j].ID })
	response := GraphResponse{OpenSpecID: openSpecID, Depth: depth, Nodes: nodes, Links: links, MaterializedAt: time.Now().UTC()}
	s.materialized[materializedKey(openSpecID, depth)] = MaterializedGraph{RootOpenSpecID: response.OpenSpecID, Depth: response.Depth, Nodes: response.Nodes, Links: response.Links, MaterializedAt: response.MaterializedAt}
	return response
}

func (s *Store) RefreshAllMaterialized(defaultDepth int) {
	s.mu.RLock()
	openSpecs := map[string]bool{}
	for _, node := range s.nodes {
		if node.Type == NodeOpenSpec {
			openSpecs[node.ExternalID] = true
		}
	}
	s.mu.RUnlock()
	for openSpecID := range openSpecs {
		s.RefreshMaterialized(openSpecID, defaultDepth)
	}
}

func (s *Store) Counts() (int, int) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return len(s.nodes), len(s.links)
}

func nodeKey(workspaceID string, nodeType NodeType, externalID string) string {
	return fmt.Sprintf("%s:%s:%s", workspaceID, nodeType, externalID)
}

func linkKey(from, to string, relation Relation, sourceEvent string) string {
	return fmt.Sprintf("%s:%s:%s:%s", from, to, relation, sourceEvent)
}

func materializedKey(openSpecID string, depth int) string {
	return fmt.Sprintf("%s:%d", openSpecID, depth)
}

func cloneNodePtr(node *Node) *Node {
	clone := cloneNode(node)
	return &clone
}

func cloneNode(node *Node) Node {
	out := *node
	out.Metadata = cloneMap(node.Metadata)
	return out
}

func cloneLinkPtr(link *Link) *Link {
	clone := cloneLink(link)
	return &clone
}

func cloneLink(link *Link) Link {
	out := *link
	out.Metadata = cloneMap(link.Metadata)
	return out
}

func cloneMap(in map[string]any) map[string]any {
	if in == nil {
		return nil
	}
	out := map[string]any{}
	for k, v := range in {
		out[k] = v
	}
	return out
}
