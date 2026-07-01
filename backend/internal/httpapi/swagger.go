package httpapi

import (
	_ "embed"
	"net/http"
)

// openapiSpec is a committed copy of docs/openapi.yaml (the canonical contract),
// embedded because go:embed cannot reach outside the module directory. Keep it in
// sync with `make sync-openapi` after editing the canonical file.
//
//go:embed openapi.yaml
var openapiSpec []byte

// swaggerHTML is a minimal Swagger UI page that loads the spec from /openapi.yaml.
// swagger-ui-dist is pulled from a CDN — acceptable for a personal panel.
const swaggerHTML = `<!DOCTYPE html>
<html lang="en">
<head>
  <meta charset="utf-8"/>
  <meta name="viewport" content="width=device-width, initial-scale=1"/>
  <title>Absolutely Disgusting Panel — API</title>
  <link rel="stylesheet" href="https://unpkg.com/swagger-ui-dist@5/swagger-ui.css"/>
</head>
<body>
  <div id="swagger-ui"></div>
  <script src="https://unpkg.com/swagger-ui-dist@5/swagger-ui-bundle.js" crossorigin></script>
  <script>
    window.onload = () => {
      window.ui = SwaggerUIBundle({
        url: 'openapi.yaml',
        dom_id: '#swagger-ui',
        deepLinking: true,
      })
    }
  </script>
</body>
</html>`

func (h *healthHandler) swaggerUI(w http.ResponseWriter, _ *http.Request) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write([]byte(swaggerHTML))
}

func (h *healthHandler) openapiYAML(w http.ResponseWriter, _ *http.Request) {
	w.Header().Set("Content-Type", "application/yaml")
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write(openapiSpec)
}
