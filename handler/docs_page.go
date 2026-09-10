package handler

import (
	"net/http"

	"github.com/gin-gonic/gin"
)

// docsPageHTML renders Swagger UI from CDN assets against the spec served
// at /swagger/doc.json, with a responseInterceptor that watches for a
// successful POST /auth/login and feeds the returned access_token straight
// into the Authorize dialog — no manual copy/paste of the bearer token.
const docsPageHTML = `<!DOCTYPE html>
<html lang="en">
<head>
  <meta charset="UTF-8" />
  <title>Mini Bank API Docs</title>
  <link rel="stylesheet" href="https://cdn.jsdelivr.net/npm/swagger-ui-dist@5/swagger-ui.css" />
  <style>body { margin: 0; }</style>
</head>
<body>
  <div id="swagger-ui"></div>
  <script src="https://cdn.jsdelivr.net/npm/swagger-ui-dist@5/swagger-ui-bundle.js"></script>
  <script>
    window.onload = function () {
      var ui = SwaggerUIBundle({
        url: "/swagger/doc.json",
        dom_id: "#swagger-ui",
        presets: [SwaggerUIBundle.presets.apis],
        deepLinking: true,
        persistAuthorization: true,
        responseInterceptor: function (response) {
          try {
            if (/\/auth\/login$/.test(response.url) && response.ok) {
              var body = response.body;
              if (!body && response.text) {
                body = JSON.parse(response.text);
              }
              var token = body && body.data && body.data.access_token;
              if (token) {
                ui.preauthorizeApiKey("BearerAuth", "Bearer " + token);
              }
            }
          } catch (e) {
            console.error("mini-bank: failed to auto-authorize from login response", e);
          }
          return response;
        },
      });
      window.ui = ui;
    };
  </script>
</body>
</html>`

func DocsPage(c *gin.Context) {
	c.Data(http.StatusOK, "text/html; charset=utf-8", []byte(docsPageHTML))
}
