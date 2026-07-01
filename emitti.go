// Package emitti é o SDK oficial do Emitti para Go — emissão de NFS-e.
//
// Envelopa a API REST (https://api.emitti.com.br) em métodos nativos. Todos os
// métodos recebem context.Context e devolvem *Error em respostas não-2xx.
//
//	cli := emitti.New(os.Getenv("EMITTI_API_KEY"))
//	em, err := cli.Emitir(ctx, emitti.NfseInput{
//	    Prestador: emitti.Prestador{CNPJ: "12345678000190", InscricaoMunicipal: "1122334"},
//	    Tomador:   emitti.Tomador{RazaoSocial: "Cliente X", CNPJ: "98765432000110"},
//	    Servico: emitti.Servico{
//	        CodigoMunicipio: "3550308", CodigoServico: "01.05",
//	        Discriminacao: "Plano SaaS", ValorServicos: 499.9, AliquotaISS: 2,
//	    },
//	})
package emitti

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

const defaultBaseURL = "https://api.emitti.com.br"

// Endereco do tomador (opcional).
type Endereco struct {
	TipoLogradouro string `json:"tipo_logradouro,omitempty"`
	Logradouro     string `json:"logradouro,omitempty"`
	Numero         string `json:"numero,omitempty"`
	Complemento    string `json:"complemento,omitempty"`
	Bairro         string `json:"bairro,omitempty"`
	CidadeIBGE     string `json:"cidade_ibge,omitempty"`
	UF             string `json:"uf,omitempty"`
	CEP            string `json:"cep,omitempty"`
}

// Prestador é o emitente (CNPJ que emite a nota).
type Prestador struct {
	CNPJ               string `json:"cnpj"`
	InscricaoMunicipal string `json:"inscricao_municipal"`
}

// Tomador é o destinatário do serviço.
type Tomador struct {
	RazaoSocial string    `json:"razao_social,omitempty"`
	CNPJ        string    `json:"cnpj,omitempty"`
	CPF         string    `json:"cpf,omitempty"`
	Email       string    `json:"email,omitempty"`
	Endereco    *Endereco `json:"endereco,omitempty"`
}

// Servico descreve o serviço prestado.
type Servico struct {
	CodigoMunicipio string  `json:"codigo_municipio"`
	CodigoServico   string  `json:"codigo_servico"`
	Discriminacao   string  `json:"discriminacao"`
	ValorServicos   float64 `json:"valor_servicos"`
	AliquotaISS     float64 `json:"aliquota_iss"`
	ISSRetido       bool    `json:"iss_retido,omitempty"`
	ValorDeducoes   float64 `json:"valor_deducoes,omitempty"`
}

// NfseInput é o corpo de uma emissão.
type NfseInput struct {
	ReferenciaExterna string    `json:"referencia_externa,omitempty"`
	Prestador         Prestador `json:"prestador"`
	Tomador           Tomador   `json:"tomador"`
	Servico           Servico   `json:"servico"`
}

// NfceItem é um produto da NFC-e.
type NfceItem struct {
	Codigo        string  `json:"codigo"`
	Descricao     string  `json:"descricao"`
	NCM           string  `json:"ncm"`
	CFOP          string  `json:"cfop"`
	Unidade       string  `json:"unidade,omitempty"`
	Quantidade    float64 `json:"quantidade"`
	ValorUnitario float64 `json:"valor_unitario"`
	CSOSN         string  `json:"csosn,omitempty"` // Simples Nacional (default 102)
	Origem        string  `json:"origem,omitempty"`
	EAN           string  `json:"ean,omitempty"`
}

// NfcePagamento é uma forma de pagamento da NFC-e.
type NfcePagamento struct {
	Forma string  `json:"forma"` // tPag: 01=dinheiro, 03=crédito, 17=Pix...
	Valor float64 `json:"valor"`
}

// NfceIde traz dados opcionais de identificação (série, natureza da operação).
type NfceIde struct {
	Serie            int    `json:"serie,omitempty"`
	NaturezaOperacao string `json:"natureza_operacao,omitempty"`
}

// NfceInput é o corpo de uma emissão de NFC-e (modelo 65).
type NfceInput struct {
	ReferenciaExterna string          `json:"referencia_externa,omitempty"`
	Prestador         Prestador       `json:"prestador"`
	UF                string          `json:"uf,omitempty"` // se ausente, usa a UF do emitente
	Ide               *NfceIde        `json:"ide,omitempty"`
	Itens             []NfceItem      `json:"itens"`
	Pagamentos        []NfcePagamento `json:"pagamentos"`
}

// Emissao é a resposta da API (202 com o emissao_id). Na emissão SÍNCRONA de NFC-e
// (EmitirNfceSync) os campos NumeroNFSe/CodigoVerificacao/QRCode/Contingencia vêm preenchidos.
type Emissao struct {
	EmissaoID         string         `json:"emissao_id"`
	Status            string         `json:"status"`
	ReferenciaExterna string         `json:"referencia_externa,omitempty"`
	Resultado         map[string]any `json:"resultado,omitempty"`
	NumeroNFSe        string         `json:"numero_nfse,omitempty"` // NFC-e síncrona: chave de acesso
	CodigoVerificacao string         `json:"codigo_verificacao,omitempty"`
	QRCode            string         `json:"qr_code,omitempty"`
	Contingencia      bool           `json:"contingencia,omitempty"`
}

// Error representa uma resposta não-2xx da API. Body é o JSON decodificado da
// resposta (ou a string crua, se não for JSON).
type Error struct {
	Status int
	Body   any
}

func (e *Error) Error() string {
	return fmt.Sprintf("emitti: API respondeu %d", e.Status)
}

// Client é o cliente HTTP do Emitti. Crie com New.
type Client struct {
	apiKey  string
	baseURL string
	http    *http.Client
}

// Option configura o Client em New.
type Option func(*Client)

// WithBaseURL sobrescreve a URL base (default https://api.emitti.com.br).
func WithBaseURL(u string) Option {
	return func(c *Client) { c.baseURL = strings.TrimRight(u, "/") }
}

// WithHTTPClient injeta um *http.Client customizado (timeouts, proxy, etc.).
func WithHTTPClient(h *http.Client) Option {
	return func(c *Client) { c.http = h }
}

// New cria um Client com a API key (sk_live_... ou sk_test_...).
func New(apiKey string, opts ...Option) *Client {
	c := &Client{
		apiKey:  apiKey,
		baseURL: defaultBaseURL,
		http:    &http.Client{Timeout: 30 * time.Second},
	}
	for _, o := range opts {
		o(c)
	}
	return c
}

// RequestOption ajusta uma requisição individual.
type RequestOption func(*http.Request)

// WithIdempotencyKey envia o header Idempotency-Key (recomendado em Emitir para
// evitar emissões duplicadas em retries).
func WithIdempotencyKey(key string) RequestOption {
	return func(r *http.Request) {
		if key != "" {
			r.Header.Set("Idempotency-Key", key)
		}
	}
}

func (c *Client) do(ctx context.Context, method, path string, body any, opts ...RequestOption) (*http.Response, error) {
	var rdr io.Reader
	if body != nil {
		b, err := json.Marshal(body)
		if err != nil {
			return nil, fmt.Errorf("emitti: serializando corpo: %w", err)
		}
		rdr = bytes.NewReader(b)
	}
	req, err := http.NewRequestWithContext(ctx, method, c.baseURL+path, rdr)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Bearer "+c.apiKey)
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	for _, o := range opts {
		o(req)
	}
	resp, err := c.http.Do(req)
	if err != nil {
		return nil, err
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		defer resp.Body.Close()
		raw, _ := io.ReadAll(resp.Body)
		var parsed any
		if json.Unmarshal(raw, &parsed) != nil {
			parsed = string(raw)
		}
		return nil, &Error{Status: resp.StatusCode, Body: parsed}
	}
	return resp, nil
}

func (c *Client) doEmissao(ctx context.Context, method, path string, body any, opts ...RequestOption) (*Emissao, error) {
	resp, err := c.do(ctx, method, path, body, opts...)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	var em Emissao
	if err := json.NewDecoder(resp.Body).Decode(&em); err != nil {
		return nil, fmt.Errorf("emitti: lendo resposta: %w", err)
	}
	return &em, nil
}

// Emitir emite uma NFS-e (assíncrono). Resposta 202 com o emissao_id.
func (c *Client) Emitir(ctx context.Context, in NfseInput, opts ...RequestOption) (*Emissao, error) {
	return c.doEmissao(ctx, http.MethodPost, "/v1/nfse", in, opts...)
}

// Consultar retorna o status de uma emissão.
func (c *Client) Consultar(ctx context.Context, emissaoID string) (*Emissao, error) {
	return c.doEmissao(ctx, http.MethodGet, "/v1/nfse/"+emissaoID, nil)
}

// Cancelar cancela uma NFS-e autorizada (assíncrono).
func (c *Client) Cancelar(ctx context.Context, emissaoID string) (*Emissao, error) {
	return c.doEmissao(ctx, http.MethodDelete, "/v1/nfse/"+emissaoID, nil)
}

// EmitirNfce emite uma NFC-e (modelo 65, varejo/consumidor). Assíncrono — 202 com o emissao_id.
func (c *Client) EmitirNfce(ctx context.Context, in NfceInput, opts ...RequestOption) (*Emissao, error) {
	return c.doEmissao(ctx, http.MethodPost, "/v1/nfce", in, opts...)
}

// EmitirNfceSync emite uma NFC-e de forma SÍNCRONA (PDV/baixa latência): a resposta
// já traz chave/protocolo/QR (~1-3s), sem esperar o webhook.
func (c *Client) EmitirNfceSync(ctx context.Context, in NfceInput, opts ...RequestOption) (*Emissao, error) {
	return c.doEmissao(ctx, http.MethodPost, "/v1/nfce?sync=1", in, opts...)
}

// CancelarNfce cancela uma NFC-e autorizada. Justificativa de 15 a 255 caracteres (regra SEFAZ).
func (c *Client) CancelarNfce(ctx context.Context, emissaoID, justificativa string) (*Emissao, error) {
	return c.doEmissao(ctx, http.MethodDelete, "/v1/nfce/"+emissaoID, map[string]string{"justificativa": justificativa})
}

// Substituir emite uma nova NFS-e e cancela a antiga.
func (c *Client) Substituir(ctx context.Context, emissaoID string, in NfseInput) (*Emissao, error) {
	return c.doEmissao(ctx, http.MethodPost, "/v1/nfse/"+emissaoID+"/substituicao", in)
}

// BaixarXML baixa o XML autorizado.
func (c *Client) BaixarXML(ctx context.Context, emissaoID string) (string, error) {
	resp, err := c.do(ctx, http.MethodGet, "/v1/nfse/"+emissaoID+"/xml", nil)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	b, err := io.ReadAll(resp.Body)
	return string(b), err
}

// BaixarPDF baixa o DANFE/RPS em PDF.
func (c *Client) BaixarPDF(ctx context.Context, emissaoID string) ([]byte, error) {
	resp, err := c.do(ctx, http.MethodGet, "/v1/nfse/"+emissaoID+"/pdf", nil)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	return io.ReadAll(resp.Body)
}

// BaixarPdfNfce baixa o DANFE-NFC-e (cupom) em PDF.
func (c *Client) BaixarPdfNfce(ctx context.Context, emissaoID string) ([]byte, error) {
	resp, err := c.do(ctx, http.MethodGet, "/v1/nfce/"+emissaoID+"/pdf", nil)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	return io.ReadAll(resp.Body)
}
