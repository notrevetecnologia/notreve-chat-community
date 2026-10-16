package web

import (
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"testing"
)

// Guardas reconhecidas como proteção de rota no registro (server.go) ou dentro
// do handler (auth interno). Qualquer rota /api/react/ sem uma destas é falha.
var guardasReact = []string{
	"requireSuperAdmin",
	"requireReactPerm",
	"requireReactModule",
	"requirePerm",
	"requireRole",
	"requireReactUserAccess",
	"requireReactTeamAccess",
	"requireReactSettingsAccess",
	"requireReactLGPD",
	"requireReactKnowledgeAccess",
	"canEditReactBranding",
	"reactRequireSuperAdmin",
}

// Rotas intencionalmente abertas a qualquer usuário autenticado.
var rotasReactAutenticadas = map[string]bool{
	"/api/react/session":   true,
	"/api/react/bootstrap": true,
}

// Colisões de numeração de migration anteriores à regra e imutáveis: renomear
// reaplicaria a migration em bancos existentes (schema_migrations guarda o
// filename), então o gate tolera o baseline e trava apenas colisões novas.
var maxArquivosPorNumeroLegado = map[string]int{
	"062": 2,
}

func lerArquivoTeste(t *testing.T, caminho string) string {
	t.Helper()
	conteudo, err := os.ReadFile(caminho)
	if err != nil {
		t.Fatalf("lendo %s: %v", caminho, err)
	}
	return string(conteudo)
}

func contemAlguma(texto string, alvos []string) bool {
	for _, alvo := range alvos {
		if strings.Contains(texto, alvo) {
			return true
		}
	}
	return false
}

func contemString(lista []string, alvo string) bool {
	for _, item := range lista {
		if item == alvo {
			return true
		}
	}
	return false
}

func pularString(src string, i int) int {
	quote := src[i]
	i++
	for i < len(src) {
		if quote == '`' {
			if src[i] == '`' {
				return i + 1
			}
			i++
			continue
		}
		if src[i] == '\\' {
			i += 2
			continue
		}
		if src[i] == quote {
			return i + 1
		}
		i++
	}
	return i
}

// extrairRegistros devolve o conteúdo interno de cada chamada mux.Handle(...).
func extrairRegistros(src string) []string {
	const marcador = "mux.Handle("
	var registros []string
	idx := 0
	for {
		encontrado := strings.Index(src[idx:], marcador)
		if encontrado < 0 {
			return registros
		}
		inicio := idx + encontrado + len(marcador)
		profundidade := 1
		j := inicio
		for j < len(src) && profundidade > 0 {
			switch src[j] {
			case '"', '`':
				j = pularString(src, j)
				continue
			case '(':
				profundidade++
			case ')':
				profundidade--
			}
			j++
		}
		registros = append(registros, src[inicio:j-1])
		idx = j
	}
}

var reAspas = regexp.MustCompile(`"([^"]*)"`)
var reHandler = regexp.MustCompile(`s\.(handle[A-Za-z0-9_]+)`)

func rotaDoRegistro(registro string) string {
	match := reAspas.FindStringSubmatch(registro)
	if match == nil {
		return ""
	}
	campos := strings.Fields(match[1])
	if len(campos) == 0 {
		return ""
	}
	return campos[len(campos)-1]
}

func corposDeHandlers(t *testing.T) map[string]string {
	t.Helper()
	entradas, err := os.ReadDir(".")
	if err != nil {
		t.Fatalf("lendo diretório do pacote: %v", err)
	}
	re := regexp.MustCompile(`func \(s \*Server\) (handle[A-Za-z0-9_]+)\(`)
	corpos := map[string]string{}
	for _, entrada := range entradas {
		if entrada.IsDir() || !strings.HasSuffix(entrada.Name(), ".go") || strings.HasSuffix(entrada.Name(), "_test.go") {
			continue
		}
		src := lerArquivoTeste(t, entrada.Name())
		for _, loc := range re.FindAllStringSubmatchIndex(src, -1) {
			nome := src[loc[2]:loc[3]]
			fim := len(src)
			if proximo := strings.Index(src[loc[1]:], "\nfunc "); proximo >= 0 {
				fim = loc[1] + proximo
			}
			corpos[nome] = src[loc[0]:fim]
		}
	}
	return corpos
}

func TestRotasReactTemGuarda(t *testing.T) {
	src := lerArquivoTeste(t, "server.go")
	corpos := corposDeHandlers(t)
	rotas := 0
	for _, registro := range extrairRegistros(src) {
		rota := rotaDoRegistro(registro)
		if !strings.HasPrefix(rota, "/api/react/") {
			continue
		}
		rotas++
		if contemAlguma(registro, guardasReact) || rotasReactAutenticadas[rota] {
			continue
		}
		guardaInterna := false
		for _, handler := range reHandler.FindAllStringSubmatch(registro, -1) {
			if contemAlguma(corpos[handler[1]], guardasReact) {
				guardaInterna = true
			}
		}
		if !guardaInterna {
			t.Errorf("rota %s sem guarda conhecida: %s", rota, strings.TrimSpace(registro))
		}
	}
	if rotas == 0 {
		t.Fatal("nenhuma rota /api/react/ encontrada em server.go")
	}
}

func TestControlPlaneExigeSuperAdmin(t *testing.T) {
	for _, registro := range extrairRegistros(lerArquivoTeste(t, "server.go")) {
		rota := rotaDoRegistro(registro)
		if strings.HasPrefix(rota, "/api/react/platform") && !strings.Contains(registro, "requireSuperAdmin") {
			t.Errorf("rota de control plane %s não usa requireSuperAdmin", rota)
		}
	}
	for numero, linha := range strings.Split(lerArquivoTeste(t, "saas_guard.go"), "\n") {
		if !strings.Contains(linha, "s.handleSaaSPlatform") && !strings.Contains(linha, "s.handleSaaSTenant") {
			continue
		}
		if !strings.Contains(linha, "requireSuperAdmin") {
			t.Errorf("saas_guard.go:%d despacha handler de plataforma sem requireSuperAdmin: %s", numero+1, strings.TrimSpace(linha))
		}
	}
}

func TestCatalogoDePermissoesIntegro(t *testing.T) {
	catalogo := map[string]int{}
	for _, grupo := range permCatalog {
		for _, permissao := range grupo.Perms {
			catalogo[permissao.Key]++
			if strings.TrimSpace(permissao.Key) == "" || strings.TrimSpace(permissao.Label) == "" {
				t.Errorf("permissão incompleta no módulo %s", grupo.Module)
			}
		}
	}
	for chave, quantidade := range catalogo {
		if quantidade > 1 {
			t.Errorf("permissão duplicada no permCatalog: %s (%d ocorrências)", chave, quantidade)
		}
	}
	for papel, permissoes := range roleDefaultPerms {
		for _, permissao := range permissoes {
			if catalogo[permissao] == 0 {
				t.Errorf("papel %s referencia permissão inexistente: %s", papel, permissao)
			}
		}
	}

	src := lerArquivoTeste(t, "perms.go")
	reConstante := regexp.MustCompile(`(perm[A-Za-z0-9_]+)\s*=\s*"([a-z0-9_]+)"`)
	constantes := map[string]bool{}
	for _, match := range reConstante.FindAllStringSubmatch(src, -1) {
		constantes[match[2]] = true
		if catalogo[match[2]] == 0 {
			t.Errorf("constante %s (%s) não está declarada no permCatalog", match[1], match[2])
		}
	}
	for chave := range catalogo {
		if !constantes[chave] {
			t.Errorf("permissão %s do permCatalog não tem constante declarada em perms.go", chave)
		}
	}
	if contemString(roleDefaultPerms["admin"], permPlataformaGerenciar) {
		t.Error("admin não pode receber plataforma_gerenciar")
	}
}

func TestMigrationsValidas(t *testing.T) {
	dir := filepath.Join("..", "..", "migrations")
	entradas, err := os.ReadDir(dir)
	if err != nil {
		t.Fatalf("lendo migrations: %v", err)
	}
	re := regexp.MustCompile(`^(\d{3})_[a-z0-9_]+\.sql$`)
	porNumero := map[string][]string{}
	total := 0
	for _, entrada := range entradas {
		if entrada.IsDir() || !strings.HasSuffix(entrada.Name(), ".sql") {
			continue
		}
		total++
		match := re.FindStringSubmatch(entrada.Name())
		if match == nil {
			t.Errorf("migration fora do padrão NNN_*.sql: %s", entrada.Name())
			continue
		}
		if strings.TrimSpace(lerArquivoTeste(t, filepath.Join(dir, entrada.Name()))) == "" {
			t.Errorf("migration vazia: %s", entrada.Name())
		}
		porNumero[match[1]] = append(porNumero[match[1]], entrada.Name())
	}
	if total == 0 {
		t.Fatal("nenhuma migration encontrada")
	}
	for numero, arquivos := range porNumero {
		limite := 1
		if legado, ok := maxArquivosPorNumeroLegado[numero]; ok {
			limite = legado
		}
		if len(arquivos) > limite {
			sort.Strings(arquivos)
			t.Errorf("número de migration %s reutilizado: %v", numero, arquivos)
		}
	}
}

func TestPacotesInternosReferenciados(t *testing.T) {
	base := filepath.Join("..", "..")
	entradas, err := os.ReadDir(filepath.Join(base, "internal"))
	if err != nil {
		t.Fatalf("lendo internal: %v", err)
	}
	var fontes strings.Builder
	err = filepath.WalkDir(base, func(caminho string, entrada fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return nil
		}
		if entrada.IsDir() {
			switch entrada.Name() {
			case ".git", "node_modules", "dist":
				return filepath.SkipDir
			}
			return nil
		}
		if strings.HasSuffix(entrada.Name(), ".go") {
			conteudo, _ := os.ReadFile(caminho)
			fontes.Write(conteudo)
		}
		return nil
	})
	if err != nil {
		t.Fatalf("varrendo fontes: %v", err)
	}
	prefixo := "github.com/notrevetecnologia/notreve-chat-community/internal/"
	for _, entrada := range entradas {
		if !entrada.IsDir() {
			continue
		}
		if !strings.Contains(fontes.String(), prefixo+entrada.Name()) {
			t.Errorf("pacote interno %s%s não é referenciado em nenhum arquivo Go", prefixo, entrada.Name())
		}
	}
}

func TestSemHandlersOrfaos(t *testing.T) {
	entradas, err := os.ReadDir(".")
	if err != nil {
		t.Fatalf("lendo pacote: %v", err)
	}
	definicoes := map[string]string{}
	var todasFontes strings.Builder
	reDef := regexp.MustCompile(`func \(s \*Server\) (handle[A-Za-z0-9_]+)\(`)
	for _, entrada := range entradas {
		if entrada.IsDir() || !strings.HasSuffix(entrada.Name(), ".go") {
			continue
		}
		src := lerArquivoTeste(t, entrada.Name())
		todasFontes.WriteString(src)
		if strings.HasSuffix(entrada.Name(), "_test.go") {
			continue
		}
		for _, match := range reDef.FindAllStringSubmatch(src, -1) {
			definicoes[match[1]] = entrada.Name()
		}
	}
	fontes := todasFontes.String()
	for nome, arquivo := range definicoes {
		re := regexp.MustCompile(`\b` + regexp.QuoteMeta(nome) + `\b`)
		if len(re.FindAllString(fontes, -1)) <= 1 {
			t.Errorf("handler %s (%s) definido mas nunca referenciado", nome, arquivo)
		}
	}
}

func TestMenuNavegacaoCoerente(t *testing.T) {
	src := lerArquivoTeste(t, filepath.Join("..", "..", "web", "react", "src", "app", "navigation.ts"))
	reItem := regexp.MustCompile(`\{ id: '([^']+)'[^}]*\}`)
	reGrupo := regexp.MustCompile(`group: '([^']+)'`)
	reModulo := regexp.MustCompile(`module: '([^']+)'`)
	rePermissao := regexp.MustCompile(`requiredPermission: '([^']+)'`)
	reQualquer := regexp.MustCompile(`requiredAnyPermission: \[([^\]]*)\]`)

	catalogo := map[string]bool{}
	for _, grupo := range permCatalog {
		for _, permissao := range grupo.Perms {
			catalogo[permissao.Key] = true
		}
	}
	itens := reItem.FindAllStringSubmatch(src, -1)
	if len(itens) == 0 {
		t.Fatal("nenhum item de navegação encontrado")
	}
	for _, item := range itens {
		id := item[1]
		texto := item[0]
		if !reModulo.MatchString(texto) {
			t.Errorf("item de menu %s sem module", id)
		}
		if !rePermissao.MatchString(texto) && !reQualquer.MatchString(texto) {
			t.Errorf("item de menu %s sem requiredPermission/requiredAnyPermission", id)
		}
		for _, match := range rePermissao.FindAllStringSubmatch(texto, -1) {
			if !catalogo[match[1]] {
				t.Errorf("item de menu %s usa permissão desconhecida %s", id, match[1])
			}
		}
		grupo := ""
		if match := reGrupo.FindStringSubmatch(texto); match != nil {
			grupo = match[1]
		}
		if grupo == "plataforma" {
			for _, match := range rePermissao.FindAllStringSubmatch(texto, -1) {
				if match[1] != permPlataformaGerenciar {
					t.Errorf("item %s no grupo plataforma usa %s; esperado %s", id, match[1], permPlataformaGerenciar)
				}
			}
		}
	}
}
