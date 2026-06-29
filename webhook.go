package emitti

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
)

// VerificarWebhook confere o header X-Emitti-Signature (HMAC-SHA256 do corpo cru).
// Valide SEMPRE antes de confiar no payload, usando o corpo exatamente como
// recebido (sem reserializar o JSON). A comparação é em tempo constante.
func VerificarWebhook(rawBody []byte, signature, secret string) bool {
	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write(rawBody)
	esperado := hex.EncodeToString(mac.Sum(nil))
	return hmac.Equal([]byte(signature), []byte(esperado))
}
