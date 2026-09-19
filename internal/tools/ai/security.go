package ai

import (
	"context"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"strings"
)

const (
	// DefaultMaxResponseBodyBytes limita envelopes JSON/texto antes do decode.
	DefaultMaxResponseBodyBytes int64 = 4 * 1024 * 1024
	// DefaultMaxDownloadBytes limita cada imagem remota antes da validação final.
	DefaultMaxDownloadBytes int64 = 25 * 1024 * 1024
	DefaultMaxRedirects           = 3
)

// ResourceLimitError indica que uma resposta excedeu o limite de memória ou
// transferência. Não é retryable nem elegível para fallback.
type ResourceLimitError struct {
	Resource string
	Limit    int64
}

func (e *ResourceLimitError) Error() string {
	if e == nil {
		return "recurso excedeu o limite"
	}
	return fmt.Sprintf("%s excede o limite de %d bytes", e.Resource, e.Limit)
}

// ImageURLPolicy controla a validação de URLs retornadas pelos modelos.
// AllowHTTP e AllowPrivateNetworks devem permanecer falsos em produção.
type ImageURLPolicy struct {
	AllowHTTP            bool
	AllowPrivateNetworks bool
	MaxRedirects         int
	Resolve              func(context.Context, string) ([]net.IP, error)
}

func (p ImageURLPolicy) normalized() ImageURLPolicy {
	if p.MaxRedirects <= 0 {
		p.MaxRedirects = DefaultMaxRedirects
	}
	if p.Resolve == nil {
		p.Resolve = func(ctx context.Context, host string) ([]net.IP, error) {
			return net.DefaultResolver.LookupIP(ctx, "ip", host)
		}
	}
	return p
}

func readLimitedBody(body io.Reader, contentLength, max int64, resource string) ([]byte, error) {
	if max <= 0 {
		max = DefaultMaxResponseBodyBytes
	}
	if contentLength > max {
		return nil, &ResourceLimitError{Resource: resource, Limit: max}
	}
	data, err := io.ReadAll(io.LimitReader(body, max+1))
	if err != nil {
		return nil, err
	}
	if int64(len(data)) > max {
		return nil, &ResourceLimitError{Resource: resource, Limit: max}
	}
	return data, nil
}

func validateImageURL(ctx context.Context, raw string, policy ImageURLPolicy) error {
	policy = policy.normalized()
	u, err := url.Parse(strings.TrimSpace(raw))
	if err != nil {
		return fmt.Errorf("URL de imagem inválida: %w", err)
	}
	if u.User != nil {
		return fmt.Errorf("URL de imagem com credenciais não é permitida")
	}
	if u.Scheme != "https" && !(policy.AllowHTTP && u.Scheme == "http") {
		return fmt.Errorf("URL de imagem deve usar HTTPS")
	}
	if u.Hostname() == "" {
		return fmt.Errorf("URL de imagem sem host")
	}
	ip := net.ParseIP(u.Hostname())
	if ip != nil {
		if !policy.AllowPrivateNetworks && isBlockedImageIP(ip) {
			return fmt.Errorf("URL de imagem aponta para rede privada ou endereço reservado")
		}
		return nil
	}
	ips, err := policy.Resolve(ctx, u.Hostname())
	if err != nil {
		return fmt.Errorf("não foi possível resolver o host da imagem: %w", err)
	}
	if len(ips) == 0 {
		return fmt.Errorf("host da imagem não possui endereço resolvível")
	}
	if !policy.AllowPrivateNetworks {
		for _, resolved := range ips {
			if isBlockedImageIP(resolved) {
				return fmt.Errorf("URL de imagem resolve para rede privada ou endereço reservado")
			}
		}
	}
	return nil
}

func isBlockedImageIP(ip net.IP) bool {
	if ip == nil {
		return true
	}
	ip = ip.To16()
	if ip == nil {
		return true
	}
	return ip.IsLoopback() || ip.IsPrivate() || ip.IsLinkLocalUnicast() || ip.IsLinkLocalMulticast() || ip.IsUnspecified() || ip.IsMulticast() || ip.Equal(net.ParseIP("169.254.169.254")) || ip.Equal(net.ParseIP("100.100.100.200"))
}

func (c *Client) imageURLPolicy() ImageURLPolicy {
	if c == nil {
		return ImageURLPolicy{}.normalized()
	}
	return c.URLPolicy.normalized()
}

func (c *Client) downloadHTTPClient() *http.Client {
	base := http.DefaultClient
	if c != nil && c.HTTPClient != nil {
		copy := *c.HTTPClient
		base = &copy
	}
	policy := c.imageURLPolicy()
	base.CheckRedirect = func(req *http.Request, via []*http.Request) error {
		if len(via) > policy.MaxRedirects {
			return fmt.Errorf("download de imagem excedeu o limite de %d redirects", policy.MaxRedirects)
		}
		return validateImageURL(req.Context(), req.URL.String(), policy)
	}
	return base
}
