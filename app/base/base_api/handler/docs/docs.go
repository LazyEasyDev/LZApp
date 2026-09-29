package docs

import (
	"bytes"
	"crypto/sha256"
	_ "embed"
	"encoding/base64"
	"encoding/xml"
	"fmt"
	"io"
	"net/http"
	"strings"

	"github.com/danielgtaylor/huma/v2"
	"github.com/danielgtaylor/huma/v2/adapters/humachi"
	"github.com/go-chi/chi/v5"
)

//go:embed docs_auth.js
var docsAuthScript string

func NewDocsHandler(api huma.API) http.Handler {
	renderer := chi.NewRouter()
	docsConfig := huma.DefaultConfig(api.OpenAPI().Info.Title, api.OpenAPI().Info.Version)
	docsConfig.OpenAPI = api.OpenAPI()
	docsConfig.CreateHooks = nil
	docsConfig.SchemasPath = ""
	docsConfig.OpenAPIPath = "/openapi"
	docsConfig.DocsPath = "/docs"
	docsConfig.DocsRenderer = huma.DocsRendererScalar
	docsConfig.DocsRendererConfig = map[string]any{
		"hideClientButton":   true,
		"agent":              map[string]any{"disabled": true},
		"showDeveloperTools": "never",
		"telemetry":          false,
		"theme":              "saturn",
		"darkMode":           true,
	}
	humachi.New(renderer, docsConfig)
	return withDocsAuth(renderer)
}

type docsResponse struct {
	header http.Header
	status int
	body   bytes.Buffer
}

func (response *docsResponse) Header() http.Header { return response.header }

func (response *docsResponse) WriteHeader(status int) {
	if response.status == 0 {
		response.status = status
	}
}

func (response *docsResponse) Write(data []byte) (int, error) {
	response.WriteHeader(http.StatusOK)
	return response.body.Write(data)
}

func withDocsAuth(renderer http.Handler) http.Handler {
	return http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		response := &docsResponse{header: make(http.Header)}
		renderer.ServeHTTP(response, request)
		if response.status == 0 {
			response.status = http.StatusOK
		}
		body := response.body.Bytes()
		if response.status == http.StatusOK && strings.HasPrefix(response.header.Get("Content-Type"), "text/html") {
			updated, err := injectDocsAuth(body)
			csp, cspErr := docsAuthCSP(response.header.Get("Content-Security-Policy"))
			if err != nil || cspErr != nil || response.header.Get("Content-Encoding") != "" {
				writer.Header().Set("Cache-Control", "no-store")
				http.Error(writer, "Documentation initialization failed", http.StatusInternalServerError)
				return
			}
			body = updated
			response.header.Set("Content-Security-Policy", csp)
			response.header.Set("Cache-Control", "no-store")
			response.header.Del("Content-Length")
			response.header.Del("ETag")
			response.header.Del("Last-Modified")
		}
		for name, values := range response.header {
			writer.Header()[name] = values
		}
		writer.WriteHeader(response.status)
		_, _ = writer.Write(body)
	})
}

func injectDocsAuth(body []byte) ([]byte, error) {
	decoder := xml.NewDecoder(bytes.NewReader(body))
	decoder.Strict = false
	decoder.AutoClose = xml.HTMLAutoClose
	decoder.Entity = xml.HTMLEntity
	referenceCount, loaderCount := 0, 0
	var loader xml.StartElement
	var loaderStart, loaderEnd int64
	insideLoader := false
	for {
		position := decoder.InputOffset()
		token, err := decoder.Token()
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, fmt.Errorf("read generated docs markup: %w", err)
		}
		switch element := token.(type) {
		case xml.StartElement:
			if insideLoader {
				return nil, fmt.Errorf("unexpected nested Scalar loader content")
			}
			if element.Name.Local != "script" {
				continue
			}
			for _, attribute := range element.Attr {
				if attribute.Name.Local == "id" && attribute.Value == "api-reference" {
					referenceCount++
				}
				if attribute.Name.Local == "src" && strings.Contains(attribute.Value, "/@scalar/api-reference@") {
					loaderCount++
					loader, loaderStart, insideLoader = element, position, true
				}
			}
		case xml.CharData:
			if insideLoader && len(bytes.TrimSpace(element)) != 0 {
				return nil, fmt.Errorf("unexpected inline Scalar loader content")
			}
		case xml.EndElement:
			if insideLoader && element.Name.Local == "script" {
				loaderEnd, insideLoader = decoder.InputOffset(), false
			}
		}
	}
	if referenceCount != 1 || loaderCount != 1 || loaderEnd <= loaderStart || insideLoader {
		return nil, fmt.Errorf("expected one Huma Scalar reference and loader")
	}
	for index := range loader.Attr {
		switch loader.Attr[index].Name.Local {
		case "src":
			loader.Attr[index].Name.Local = "data-src"
		case "id", "type":
			return nil, fmt.Errorf("unexpected Scalar loader attributes")
		}
	}
	loader.Attr = append(loader.Attr,
		xml.Attr{Name: xml.Name{Local: "id"}, Value: "lzapp-scalar-loader"},
		xml.Attr{Name: xml.Name{Local: "type"}, Value: "application/json"},
	)
	var rewritten bytes.Buffer
	rewritten.Write(body[:loaderStart])
	encoder := xml.NewEncoder(&rewritten)
	if err := encoder.EncodeToken(loader); err != nil {
		return nil, err
	}
	if err := encoder.EncodeToken(loader.End()); err != nil {
		return nil, err
	}
	if err := encoder.Flush(); err != nil {
		return nil, err
	}
	rewritten.WriteString("<script>")
	rewritten.WriteString(docsAuthScript)
	rewritten.WriteString("</script>")
	rewritten.Write(body[loaderEnd:])
	return rewritten.Bytes(), nil
}

func docsAuthCSP(policy string) (string, error) {
	digest := sha256.Sum256([]byte(docsAuthScript))
	hash := "'sha256-" + base64.StdEncoding.EncodeToString(digest[:]) + "'"
	directives := strings.Split(policy, ";")
	found := false
	for index, directive := range directives {
		parts := strings.Fields(directive)
		if len(parts) != 0 && (parts[0] == "script-src" || parts[0] == "script-src-elem") {
			directives[index] = strings.TrimSpace(directive) + " " + hash
			found = true
		}
	}
	if !found {
		return "", fmt.Errorf("docs CSP is missing a script source policy")
	}
	return strings.Join(directives, ";"), nil
}
