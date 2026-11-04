# Manual: Compilando e Distribuindo seu CLI em Go

Guia prático para transformar seu app Go em um **binário único** e instalá-lo em várias máquinas Linux.

---

## Sumário

1. [Pré-requisitos](#1-pré-requisitos)
2. [Estrutura recomendada do projeto](#2-estrutura-recomendada-do-projeto)
3. [Compilação básica](#3-compilação-básica)
4. [Binário estático e otimizado](#4-binário-estático-e-otimizado)
5. [Injetando versão no binário](#5-injetando-versão-no-binário)
6. [Cross-compilation (outras arquiteturas)](#6-cross-compilation-outras-arquiteturas)
7. [Automatizando com Makefile](#7-automatizando-com-makefile)
8. [Checksums e verificação](#8-checksums-e-verificação)
9. [Instalação nas máquinas](#9-instalação-nas-máquinas)
10. [Automatizando releases com GoReleaser](#10-automatizando-releases-com-goreleaser)
11. [CI/CD com GitHub Actions](#11-cicd-com-github-actions)
12. [Solução de problemas](#12-solução-de-problemas)
13. [Checklist final](#13-checklist-final)

---

## 1. Pré-requisitos

Instale o Go na máquina de **build** (as máquinas de destino **não** precisam do Go).

```bash
# Verificar se já está instalado
go version

# Instalação manual (exemplo; confira a versão atual em https://go.dev/dl/)
wget https://go.dev/dl/go1.23.0.linux-amd64.tar.gz
sudo rm -rf /usr/local/go
sudo tar -C /usr/local -xzf go1.23.0.linux-amd64.tar.gz

# Adicionar ao PATH (coloque no ~/.bashrc ou ~/.zshrc)
export PATH=$PATH:/usr/local/go/bin
```

Confirme:

```bash
go version
```

---

## 2. Estrutura recomendada do projeto

```
meucli/
├── go.mod
├── go.sum
├── main.go              # ou cmd/meucli/main.go
├── internal/            # código interno do app
├── Makefile
├── .goreleaser.yaml     # opcional
└── README.md
```

Se ainda não tem o módulo inicializado:

```bash
go mod init github.com/seu-usuario/meucli
go mod tidy
```

> `go mod tidy` baixa as dependências e remove as não utilizadas. Rode sempre antes de compilar.

---

## 3. Compilação básica

Na raiz do projeto:

```bash
go build -o meucli .
```

Se o `main.go` estiver em `cmd/meucli/`:

```bash
go build -o meucli ./cmd/meucli
```

Teste:

```bash
./meucli --help
```

Esse binário funciona, mas ainda não é o ideal para distribuição (pode depender da glibc e é maior que o necessário).

---

## 4. Binário estático e otimizado

Use este comando para gerar um binário **autocontido** e **menor**:

```bash
CGO_ENABLED=0 go build -trimpath -ldflags="-s -w" -o meucli .
```

| Opção              | O que faz                                                                                               |
| ------------------ | ------------------------------------------------------------------------------------------------------- |
| `CGO_ENABLED=0`    | Desativa o CGO. O binário fica **100% estático**, sem depender da glibc. Roda em qualquer distro Linux. |
| `-trimpath`        | Remove caminhos locais do binário (build reproduzível e mais limpo).                                    |
| `-ldflags="-s -w"` | Remove tabela de símbolos e informações de debug. Reduz bastante o tamanho.                             |

### Verificando se ficou estático

```bash
file meucli
# esperado: ... statically linked ...

ldd meucli
# esperado: "not a dynamic executable"
```

### Compressão opcional com UPX

Reduz ainda mais o tamanho (30-60%), com um pequeno custo no tempo de inicialização:

```bash
sudo apt install upx
upx --best --lzma meucli
```

> Atenção: alguns antivírus e ferramentas de segurança marcam binários comprimidos com UPX como suspeitos. Em ambiente corporativo, avalie se vale a pena.

---

## 5. Injetando versão no binário

Útil para o comando `meucli --version`. No código, declare variáveis:

```go
package main

import "fmt"

var (
	version = "dev"
	commit  = "none"
	date    = "unknown"
)

func main() {
	fmt.Printf("meucli %s (commit %s, build %s)\n", version, commit, date)
}
```

Compile injetando os valores:

```bash
VERSION=1.0.0
COMMIT=$(git rev-parse --short HEAD)
DATE=$(date -u +%Y-%m-%dT%H:%M:%SZ)

CGO_ENABLED=0 go build -trimpath \
  -ldflags="-s -w -X main.version=${VERSION} -X main.commit=${COMMIT} -X main.date=${DATE}" \
  -o meucli .
```

Se as variáveis estiverem em outro pacote, use o caminho completo:
`-X github.com/seu-usuario/meucli/internal/version.Version=${VERSION}`

---

## 6. Cross-compilation (outras arquiteturas)

O Go compila para outras plataformas **sem instalar nada extra**. Basta definir `GOOS` e `GOARCH`.

```bash
# Linux x86_64 (servidores e PCs comuns)
CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -trimpath -ldflags="-s -w" -o dist/meucli-linux-amd64 .

# Linux ARM64 (Raspberry Pi 4/5, servidores ARM, AWS Graviton)
CGO_ENABLED=0 GOOS=linux GOARCH=arm64 go build -trimpath -ldflags="-s -w" -o dist/meucli-linux-arm64 .

# Linux ARM 32 bits (Raspberry Pi antigos)
CGO_ENABLED=0 GOOS=linux GOARCH=arm GOARM=7 go build -trimpath -ldflags="-s -w" -o dist/meucli-linux-armv7 .

# Linux 32 bits (raro)
CGO_ENABLED=0 GOOS=linux GOARCH=386 go build -trimpath -ldflags="-s -w" -o dist/meucli-linux-386 .
```

### Como descobrir a arquitetura da máquina de destino

```bash
uname -m
```

| Saída de `uname -m` | Use `GOARCH`        |
| ------------------- | ------------------- |
| `x86_64`            | `amd64`             |
| `aarch64` / `arm64` | `arm64`             |
| `armv7l`            | `arm` com `GOARM=7` |
| `i686` / `i386`     | `386`               |

### Ver todas as combinações suportadas

```bash
go tool dist list
```

---

## 7. Automatizando com Makefile

Crie um `Makefile` na raiz:

```makefile
APP      := meucli
VERSION  ?= $(shell git describe --tags --always --dirty 2>/dev/null || echo dev)
COMMIT   := $(shell git rev-parse --short HEAD 2>/dev/null || echo none)
DATE     := $(shell date -u +%Y-%m-%dT%H:%M:%SZ)
LDFLAGS  := -s -w -X main.version=$(VERSION) -X main.commit=$(COMMIT) -X main.date=$(DATE)
BUILD    := CGO_ENABLED=0 go build -trimpath -ldflags="$(LDFLAGS)"

.PHONY: build build-all clean test checksums

build:
	$(BUILD) -o dist/$(APP) .

build-all: clean
	GOOS=linux GOARCH=amd64         $(BUILD) -o dist/$(APP)-linux-amd64 .
	GOOS=linux GOARCH=arm64         $(BUILD) -o dist/$(APP)-linux-arm64 .
	GOOS=linux GOARCH=arm GOARM=7   $(BUILD) -o dist/$(APP)-linux-armv7 .

checksums:
	cd dist && sha256sum $(APP)-* > checksums.txt

test:
	go test ./...

clean:
	rm -rf dist
```

Uso:

```bash
make build          # compila só para a máquina atual
make build-all      # compila para todas as arquiteturas
make checksums      # gera os hashes SHA256
make test           # roda os testes
```

> Importante: o Makefile exige **TAB** (não espaços) na indentação dos comandos.

---

## 8. Checksums e verificação

Gere hashes para garantir a integridade dos binários distribuídos:

```bash
cd dist
sha256sum meucli-* > checksums.txt
```

Na máquina de destino, após baixar:

```bash
sha256sum -c checksums.txt --ignore-missing
```

---

## 9. Instalação nas máquinas

### Opção A: cópia manual (SCP)

```bash
# Da máquina de build para o servidor
scp dist/meucli-linux-amd64 usuario@servidor:/tmp/meucli

# No servidor
ssh usuario@servidor
sudo install -m 755 /tmp/meucli /usr/local/bin/meucli
meucli --version
```

O comando `install -m 755` copia o arquivo e já define a permissão de execução.

### Opção B: download direto via curl (GitHub Releases)

```bash
curl -L -o meucli https://github.com/seu-usuario/meucli/releases/download/v1.0.0/meucli-linux-amd64
chmod +x meucli
sudo mv meucli /usr/local/bin/
```

### Opção C: script de instalação universal

Salve como `install.sh` e hospede no repositório:

```bash
#!/usr/bin/env bash
set -euo pipefail

REPO="seu-usuario/meucli"
APP="meucli"
VERSION="${VERSION:-latest}"

# Detecta arquitetura
case "$(uname -m)" in
  x86_64)        ARCH="amd64" ;;
  aarch64|arm64) ARCH="arm64" ;;
  armv7l)        ARCH="armv7" ;;
  *) echo "Arquitetura não suportada: $(uname -m)"; exit 1 ;;
esac

# Monta a URL
if [ "$VERSION" = "latest" ]; then
  URL="https://github.com/${REPO}/releases/latest/download/${APP}-linux-${ARCH}"
else
  URL="https://github.com/${REPO}/releases/download/${VERSION}/${APP}-linux-${ARCH}"
fi

echo "Baixando ${APP} (${ARCH})..."
TMP="$(mktemp)"
curl -fsSL "$URL" -o "$TMP"

sudo install -m 755 "$TMP" "/usr/local/bin/${APP}"
rm -f "$TMP"

echo "Instalado: $(${APP} --version)"
```

Uso em qualquer máquina:

```bash
curl -fsSL https://raw.githubusercontent.com/seu-usuario/meucli/main/install.sh | bash
```

### Opção D: instalar sem sudo (somente para o usuário)

```bash
mkdir -p ~/.local/bin
cp meucli ~/.local/bin/
chmod +x ~/.local/bin/meucli

# Garanta que está no PATH (adicione ao ~/.bashrc)
export PATH="$HOME/.local/bin:$PATH"
```

### Opção E: instalar direto pelo Go (se a máquina já tem Go)

```bash
go install github.com/seu-usuario/meucli@latest
```

O binário vai para `$(go env GOPATH)/bin` (normalmente `~/go/bin`).

### Instalando em várias máquinas de uma vez (Ansible)

```yaml
- hosts: servidores
  become: true
  tasks:
    - name: Instalar meucli
      ansible.builtin.copy:
        src: dist/meucli-linux-amd64
        dest: /usr/local/bin/meucli
        mode: "0755"
```

```bash
ansible-playbook -i inventario.ini instalar.yml
```

---

## 10. Automatizando releases com GoReleaser

O **GoReleaser** faz tudo de uma vez: compila para várias plataformas, gera checksums, cria o release no GitHub e pode gerar pacotes `.deb` e `.rpm`.

### Instalação

```bash
go install github.com/goreleaser/goreleaser/v2@latest
# ou via snap: sudo snap install goreleaser --classic
```

### Configuração (`.goreleaser.yaml`)

```bash
goreleaser init
```

Ou crie manualmente:

```yaml
version: 2

project_name: meucli

builds:
  - main: .
    binary: meucli
    env:
      - CGO_ENABLED=0
    goos:
      - linux
    goarch:
      - amd64
      - arm64
      - arm
    goarm:
      - "7"
    flags:
      - -trimpath
    ldflags:
      - -s -w
      - -X main.version={{.Version}}
      - -X main.commit={{.Commit}}
      - -X main.date={{.Date}}

archives:
  - formats: [tar.gz]
    name_template: "{{ .ProjectName }}_{{ .Version }}_{{ .Os }}_{{ .Arch }}"

checksum:
  name_template: checksums.txt

nfpms:
  - package_name: meucli
    formats:
      - deb
      - rpm
    maintainer: "Seu Nome <voce@email.com>"
    description: "Descrição do meu CLI"
    license: MIT
```

### Testar localmente (sem publicar)

```bash
goreleaser release --snapshot --clean
```

Os arquivos ficam em `dist/`.

### Publicar uma versão

```bash
git tag -a v1.0.0 -m "Primeira versão"
git push origin v1.0.0

export GITHUB_TOKEN="seu_token"
goreleaser release --clean
```

Com os pacotes `.deb`, a instalação em Debian/Ubuntu fica:

```bash
sudo dpkg -i meucli_1.0.0_linux_amd64.deb
```

---

## 11. CI/CD com GitHub Actions

Crie `.github/workflows/release.yml` para publicar automaticamente ao criar uma tag:

```yaml
name: release

on:
  push:
    tags:
      - "v*"

permissions:
  contents: write

jobs:
  goreleaser:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v4
        with:
          fetch-depth: 0

      - uses: actions/setup-go@v5
        with:
          go-version: stable

      - uses: goreleaser/goreleaser-action@v6
        with:
          version: "~> v2"
          args: release --clean
        env:
          GITHUB_TOKEN: ${{ secrets.GITHUB_TOKEN }}
```

Fluxo final: você faz `git tag v1.0.1 && git push --tags` e o GitHub compila e publica tudo sozinho.

---

## 12. Solução de problemas

| Problema                                            | Causa provável                  | Solução                                                                                                                |
| --------------------------------------------------- | ------------------------------- | ---------------------------------------------------------------------------------------------------------------------- |
| `GLIBC_2.xx not found` ao rodar em outra máquina    | Binário compilado com CGO ativo | Compile com `CGO_ENABLED=0`                                                                                            |
| `exec format error`                                 | Arquitetura errada              | Confira `uname -m` e use o `GOARCH` correto                                                                            |
| `Permission denied` ao executar                     | Sem permissão de execução       | `chmod +x meucli`                                                                                                      |
| `command not found` após instalar                   | Diretório fora do `PATH`        | Mova para `/usr/local/bin` ou ajuste o `PATH`                                                                          |
| `cannot find package`                               | Dependências ausentes           | `go mod tidy`                                                                                                          |
| Binário muito grande                                | Símbolos de debug incluídos     | Use `-ldflags="-s -w"` (e opcionalmente UPX)                                                                           |
| Preciso de CGO (ex.: SQLite com `mattn/go-sqlite3`) | Lib depende de C                | Troque por alternativa pura em Go (ex.: `modernc.org/sqlite`) ou use `musl-gcc` com `-ldflags '-extldflags "-static"'` |
| Erro de DNS em binário estático                     | Resolver puro Go vs. do sistema | Use `-tags netgo` ou defina `GODEBUG=netdns=go`                                                                        |

---

## 13. Checklist final

Antes de distribuir uma versão:

- [ ] `go mod tidy` executado
- [ ] `go vet ./...` sem avisos
- [ ] `go test ./...` passando
- [ ] Compilado com `CGO_ENABLED=0`
- [ ] Flags `-trimpath` e `-ldflags="-s -w"` aplicadas
- [ ] Versão injetada e `meucli --version` funcionando
- [ ] Testado em ao menos uma máquina limpa (sem Go instalado)
- [ ] Binários gerados para todas as arquiteturas necessárias
- [ ] `checksums.txt` gerado
- [ ] Tag Git criada (`v1.0.0`)
- [ ] Release publicado e script de instalação testado

---

## Referência rápida

```bash
# Build de produção (máquina atual)
CGO_ENABLED=0 go build -trimpath -ldflags="-s -w" -o meucli .

# Build para ARM64
CGO_ENABLED=0 GOOS=linux GOARCH=arm64 go build -trimpath -ldflags="-s -w" -o meucli-arm64 .

# Instalar
sudo install -m 755 meucli /usr/local/bin/meucli

# Release completo
git tag v1.0.0 && git push --tags && goreleaser release --clean
```