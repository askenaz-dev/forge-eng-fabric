package runtime

import (
	"crypto/sha256"
	"encoding/base64"
	"errors"
)

// KMS is the credential encryption interface required by the
// `byo-runtime-onboarding` spec ("credentials MUST be encrypted at rest using
// KMS scoped per Tenant"). Production wires this to Cloud KMS; tests use
// `FakeKMS` which encodes the plaintext via XOR with a per-tenant key so the
// store never holds the literal credential.
type KMS interface {
	Encrypt(tenantID, plaintext string) (cipherB64, keyRef string, err error)
}

var ErrEncryptionRequired = errors.New("encryption_required")

type FakeKMS struct {
	KeyRefPrefix string
}

func NewFakeKMS() *FakeKMS { return &FakeKMS{KeyRefPrefix: "projects/forge/locations/global/keyRings/byo/cryptoKeys/"} }

func (f *FakeKMS) Encrypt(tenantID, plaintext string) (string, string, error) {
	if plaintext == "" {
		return "", "", errors.New("empty_credential")
	}
	if tenantID == "" {
		return "", "", errors.New("missing_tenant")
	}
	keyRef := f.KeyRefPrefix + tenantID
	keyDigest := sha256.Sum256([]byte("fake-kms:" + tenantID))
	cipher := make([]byte, len(plaintext))
	for i := 0; i < len(plaintext); i++ {
		cipher[i] = plaintext[i] ^ keyDigest[i%len(keyDigest)]
	}
	return base64.StdEncoding.EncodeToString(cipher), keyRef, nil
}
