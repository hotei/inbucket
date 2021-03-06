package web

import (
	"fmt"
	"github.com/jhillyerd/inbucket/config"
	"html/template"
	"io/ioutil"
	"net/http"
)

func RootIndex(w http.ResponseWriter, req *http.Request, ctx *Context) (err error) {
	greeting, err := ioutil.ReadFile(config.GetWebConfig().GreetingFile)
	if err != nil {
		return fmt.Errorf("Failed to load greeting: %v", err)
	}

	return RenderTemplate("root/index.html", w, map[string]interface{}{
		"ctx":      ctx,
		"greeting": template.HTML(string(greeting)),
	})
}

func RootStatus(w http.ResponseWriter, req *http.Request, ctx *Context) (err error) {
	retentionMinutes := config.GetDataStoreConfig().RetentionMinutes
	smtpListener := fmt.Sprintf("%s:%d", config.GetSmtpConfig().Ip4address.String(),
		config.GetSmtpConfig().Ip4port)
	pop3Listener := fmt.Sprintf("%s:%d", config.GetPop3Config().Ip4address.String(),
		config.GetPop3Config().Ip4port)
	webListener := fmt.Sprintf("%s:%d", config.GetWebConfig().Ip4address.String(),
		config.GetWebConfig().Ip4port)
	return RenderTemplate("root/status.html", w, map[string]interface{}{
		"ctx":              ctx,
		"version":          config.VERSION,
		"buildDate":        config.BUILD_DATE,
		"retentionMinutes": retentionMinutes,
		"smtpListener":     smtpListener,
		"pop3Listener":     pop3Listener,
		"webListener":      webListener,
	})
}
