# emitti-go

SDK oficial do Emitti para Go (>= 1.21). Envelopa a API REST em métodos nativos,
com `context.Context`, sem dependências externas (stdlib pura).

```bash
go get github.com/emittibr/emitti-go@latest
```

```go
package main

import (
	"context"
	"log"
	"os"

	emitti "github.com/emittibr/emitti-go"
)

func main() {
	cli := emitti.New(os.Getenv("EMITTI_API_KEY"))
	ctx := context.Background()

	nota, err := cli.Emitir(ctx, emitti.NfseInput{
		Prestador: emitti.Prestador{CNPJ: "12345678000190", InscricaoMunicipal: "1122334"},
		Tomador:   emitti.Tomador{RazaoSocial: "Cliente X", CNPJ: "98765432000110"},
		Servico: emitti.Servico{
			CodigoMunicipio: "3550308", CodigoServico: "01.05",
			Discriminacao: "Plano SaaS", ValorServicos: 499.9, AliquotaISS: 2,
		},
	}, emitti.WithIdempotencyKey("pedido-00871"))
	if err != nil {
		log.Fatal(err)
	}
	// nota.EmissaoID, nota.Status == "QUEUED"

	_, _ = cli.Consultar(ctx, nota.EmissaoID)
	_, _ = cli.Cancelar(ctx, nota.EmissaoID)
	_, _ = cli.Substituir(ctx, nota.EmissaoID, emitti.NfseInput{ /* nova nota */ })
	xml, _ := cli.BaixarXML(ctx, nota.EmissaoID)
	pdf, _ := cli.BaixarPDF(ctx, nota.EmissaoID) // []byte
	_ = xml
	_ = pdf
}
```

## NFC-e (modelo 65, varejo/consumidor)

IE e CSC ficam no cadastro do emitente; no corpo vão só itens e pagamentos.

```go
cupom, err := cli.EmitirNfce(ctx, emitti.NfceInput{
	Prestador:  emitti.Prestador{CNPJ: "12345678000190"},
	UF:         "RS", // opcional — default: UF do emitente
	Itens: []emitti.NfceItem{{
		Codigo: "SKU1", Descricao: "Camiseta", NCM: "61091000",
		CFOP: "5102", Quantidade: 1, ValorUnitario: 59.9,
	}},
	Pagamentos: []emitti.NfcePagamento{{Forma: "01", Valor: 59.9}}, // 01=dinheiro, 03=crédito, 17=Pix
})
pdf, _ := cli.BaixarPdfNfce(ctx, cupom.EmissaoID) // []byte (DANFE-NFC-e)
// justificativa de 15 a 255 caracteres (regra SEFAZ)
_, _ = cli.CancelarNfce(ctx, cupom.EmissaoID, "Cancelamento a pedido do cliente no PDV.")
```

## Erros

Respostas não-2xx voltam como `*emitti.Error` (com `Status` e `Body`):

```go
em, err := cli.Consultar(ctx, id)
var apiErr *emitti.Error
if errors.As(err, &apiErr) {
	log.Printf("status=%d body=%v", apiErr.Status, apiErr.Body)
}
```

## Configuração

```go
cli := emitti.New(apiKey,
	emitti.WithBaseURL("https://api.emitti.com.br"), // default
	emitti.WithHTTPClient(&http.Client{Timeout: 10 * time.Second}),
)
```

## Webhooks

Valide a assinatura com o corpo **cru** (antes de desserializar):

```go
ok := emitti.VerificarWebhook(rawBody, r.Header.Get("X-Emitti-Signature"), os.Getenv("EMITTI_WEBHOOK_SECRET"))
if !ok {
	http.Error(w, "assinatura inválida", http.StatusBadRequest)
	return
}
```

## Desenvolvimento

```bash
go vet ./...
go test ./...
```

## Release

Versões são tags git semânticas (`vX.Y.Z`):

```bash
git tag v0.1.0 && git push origin v0.1.0
```
