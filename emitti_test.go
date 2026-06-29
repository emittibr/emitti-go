package emitti

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func assinar(body, secret string) string {
	m := hmac.New(sha256.New, []byte(secret))
	m.Write([]byte(body))
	return hex.EncodeToString(m.Sum(nil))
}

func TestVerificarWebhook(t *testing.T) {
	secret := "whsec_teste"
	body := `{"event_type":"nfse.authorized","data":{"emissao_id":"emi_1"}}`
	sig := assinar(body, secret)

	if !VerificarWebhook([]byte(body), sig, secret) {
		t.Fatal("assinatura válida deveria passar")
	}
	if VerificarWebhook([]byte(body), "deadbeef", secret) {
		t.Fatal("assinatura inválida deveria falhar")
	}
}

func TestEmitir(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost || r.URL.Path != "/v1/nfse" {
			t.Errorf("rota inesperada: %s %s", r.Method, r.URL.Path)
		}
		if got := r.Header.Get("Authorization"); got != "Bearer sk_test_x" {
			t.Errorf("Authorization = %q", got)
		}
		if got := r.Header.Get("Idempotency-Key"); got != "idem-1" {
			t.Errorf("Idempotency-Key = %q", got)
		}
		w.WriteHeader(http.StatusAccepted)
		_ = json.NewEncoder(w).Encode(map[string]any{"emissao_id": "emi_9", "status": "QUEUED"})
	}))
	defer srv.Close()

	cli := New("sk_test_x", WithBaseURL(srv.URL))
	em, err := cli.Emitir(context.Background(), NfseInput{
		Prestador: Prestador{CNPJ: "12345678000190", InscricaoMunicipal: "1122334"},
		Tomador:   Tomador{RazaoSocial: "Cliente X", CNPJ: "98765432000110"},
		Servico: Servico{
			CodigoMunicipio: "3550308", CodigoServico: "01.05",
			Discriminacao: "Plano SaaS", ValorServicos: 499.9, AliquotaISS: 2,
		},
	}, WithIdempotencyKey("idem-1"))
	if err != nil {
		t.Fatalf("Emitir: %v", err)
	}
	if em.EmissaoID != "emi_9" || em.Status != "QUEUED" {
		t.Fatalf("resposta inesperada: %+v", em)
	}
}

func TestErroAPI(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnprocessableEntity)
		_ = json.NewEncoder(w).Encode(map[string]any{"erro": "cnpj_invalido"})
	}))
	defer srv.Close()

	cli := New("sk_test_x", WithBaseURL(srv.URL))
	_, err := cli.Consultar(context.Background(), "emi_x")
	if err == nil {
		t.Fatal("esperava erro")
	}
	apiErr, ok := err.(*Error)
	if !ok || apiErr.Status != http.StatusUnprocessableEntity {
		t.Fatalf("erro inesperado: %v", err)
	}
}
