package middlewares

import (
	"github.com/containous/traefik/types"
	"github.com/unrolled/secure"
)

// NewSecure constructs a new Secure instance with supplied options.
func NewSecure(headers types.Headers) *secure.Secure {
	opt := secure.Options{
		AllowedHosts:            headers.AllowedHosts,
		HostsProxyHeaders:       headers.HostsProxyHeaders,
		SSLRedirect:             headers.SSLRedirect,
		SSLTemporaryRedirect:    headers.SSLTemporaryRedirect,
		SSLHost:                 headers.SSLHost,
		SSLProxyHeaders:         headers.SSLProxyHeaders,
		STSSeconds:              headers.STSSeconds,
		STSIncludeSubdomains:    headers.STSIncludeSubdomains,
		STSPreload:              headers.STSPreload,
		ForceSTSHeader:          headers.ForceSTSHeader,
		FrameDeny:               headers.FrameDeny,
		CustomFrameOptionsValue: headers.CustomFrameOptionsValue,
		ContentTypeNosniff:      headers.ContentTypeNosniff,
		BrowserXssFilter:        headers.BrowserXSSFilter,
		ContentSecurityPolicy:   headers.ContentSecurityPolicy,
		PublicKey:               headers.PublicKey,
		ReferrerPolicy:          headers.ReferrerPolicy,
		IsDevelopment:           headers.IsDevelopment,
	}
	return secure.New(opt)
}
