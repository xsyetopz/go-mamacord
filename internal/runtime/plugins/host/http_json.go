package pluginhost

import (
	"context"
	"crypto/tls"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"mime"
	"net"
	"net/http"
	"net/netip"
	"net/url"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/xsyetopz/go-mamacord/internal/runtime/plugins/contract"
)

const maxHTTPJSONResponseBytes int64 = 64 * 1024

type httpJSONClient struct{ client *http.Client }

type httpJSONResolver interface {
	LookupNetIP(context.Context, string, string) ([]netip.Addr, error)
}

type httpJSONOptions struct {
	Timeout  time.Duration
	Resolver httpJSONResolver
}

func newHTTPJSONClient(options httpJSONOptions) (*httpJSONClient, error) {
	if options.Timeout <= 0 {
		return nil, errors.New("HTTP JSON timeout must be positive")
	}
	resolver := options.Resolver
	if resolver == nil {
		resolver = net.DefaultResolver
	}
	transport := http.DefaultTransport.(*http.Transport).Clone()
	transport.Proxy = nil
	transport.DialContext = secureHTTPJSONDialContext(resolver)
	transport.TLSClientConfig = &tls.Config{MinVersion: tls.VersionTLS12}
	transport.ResponseHeaderTimeout = options.Timeout
	transport.MaxResponseHeaderBytes = 64 * 1024
	client := &http.Client{Transport: transport, Timeout: options.Timeout, CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }}
	return &httpJSONClient{client: client}, nil
}

func (client *httpJSONClient) GetJSON(ctx context.Context, rawURL string, maxBytes int64) (contract.Value, bool, error) {
	if client == nil || client.client == nil {
		return contract.Value{}, false, errors.New("HTTP JSON client is not initialized")
	}
	if maxBytes < 1 || maxBytes > maxHTTPJSONResponseBytes {
		return contract.Value{}, false, fmt.Errorf("response limit must be between 1 and %d", maxHTTPJSONResponseBytes)
	}
	parsed, err := url.Parse(rawURL)
	if err != nil || parsed.Scheme != "https" || parsed.Hostname() == "" || parsed.User != nil || parsed.Fragment != "" {
		return contract.Value{}, false, errors.New("HTTP JSON URL is invalid")
	}
	if port := parsed.Port(); port != "" && port != "443" {
		return contract.Value{}, false, errors.New("HTTP JSON URL port is invalid")
	}
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, parsed.String(), nil)
	if err != nil {
		return contract.Value{}, false, err
	}
	request.Header.Set("Accept", "application/json")
	response, err := client.client.Do(request)
	if err != nil {
		if ctx.Err() != nil {
			return contract.Value{}, false, ctx.Err()
		}
		return contract.Value{}, false, nil
	}
	defer response.Body.Close()
	if response.StatusCode < 200 || response.StatusCode >= 300 {
		return contract.Value{}, false, nil
	}
	mediaType, _, err := mime.ParseMediaType(response.Header.Get("Content-Type"))
	if err != nil || mediaType != "application/json" && !strings.HasSuffix(mediaType, "+json") {
		return contract.Value{}, false, nil
	}
	limited := io.LimitReader(response.Body, maxBytes+1)
	payload, err := io.ReadAll(limited)
	if err != nil {
		if ctx.Err() != nil {
			return contract.Value{}, false, ctx.Err()
		}
		return contract.Value{}, false, nil
	}
	if int64(len(payload)) > maxBytes {
		return contract.Value{}, false, nil
	}
	if err := validateUniqueHTTPJSONKeys(payload); err != nil {
		return contract.Value{}, false, nil
	}
	decoder := json.NewDecoder(strings.NewReader(string(payload)))
	decoder.UseNumber()
	var raw any
	if err := decoder.Decode(&raw); err != nil {
		return contract.Value{}, false, nil
	}
	var extra any
	if err := decoder.Decode(&extra); err != io.EOF {
		return contract.Value{}, false, nil
	}
	value, err := convertHTTPJSON(raw, 0, new(int))
	if err != nil {
		return contract.Value{}, false, nil
	}
	if err := value.Validate(); err != nil {
		return contract.Value{}, false, nil
	}
	return value, true, nil
}

func secureHTTPJSONDialContext(resolver httpJSONResolver) func(context.Context, string, string) (net.Conn, error) {
	return func(ctx context.Context, network, address string) (net.Conn, error) {
		host, port, err := net.SplitHostPort(address)
		if err != nil {
			return nil, err
		}
		addresses, err := resolver.LookupNetIP(ctx, "ip", host)
		if err != nil {
			return nil, fmt.Errorf("resolve HTTP host: %w", err)
		}
		if err := validateHTTPJSONResolvedIPs(addresses); err != nil {
			return nil, err
		}
		dialer := net.Dialer{}
		var failures []error
		for _, resolved := range addresses {
			connection, dialErr := dialer.DialContext(ctx, network, net.JoinHostPort(resolved.String(), port))
			if dialErr == nil {
				return connection, nil
			}
			failures = append(failures, dialErr)
			if ctx.Err() != nil {
				break
			}
		}
		return nil, fmt.Errorf("connect HTTP host: %w", errors.Join(failures...))
	}
}
func validateHTTPJSONResolvedIPs(addresses []netip.Addr) error {
	if len(addresses) == 0 {
		return errors.New("HTTP host resolved to no addresses")
	}
	for _, address := range addresses {
		address = address.Unmap()
		if !address.IsValid() || address.IsPrivate() || address.IsLoopback() || address.IsLinkLocalUnicast() || address.IsLinkLocalMulticast() || address.IsMulticast() || address.IsUnspecified() {
			return fmt.Errorf("HTTP host resolved to prohibited address %s", address)
		}
	}
	return nil
}
func convertHTTPJSON(raw any, depth int, items *int) (contract.Value, error) {
	if depth > contract.MaxValueDepth {
		return contract.Value{}, errors.New("JSON exceeds maximum depth")
	}
	switch value := raw.(type) {
	case nil:
		return contract.NullValue(), nil
	case bool:
		return contract.BoolValue(value), nil
	case string:
		return contract.StringValue(value), nil
	case json.Number:
		if integer, err := strconv.ParseInt(string(value), 10, 64); err == nil {
			return contract.IntValue(integer), nil
		}
		number, err := strconv.ParseFloat(string(value), 64)
		if err != nil {
			return contract.Value{}, err
		}
		return contract.FloatValue(number)
	case []any:
		*items += len(value)
		if *items > contract.MaxValueItems {
			return contract.Value{}, errors.New("JSON exceeds item limit")
		}
		converted := make([]contract.Value, len(value))
		for index, item := range value {
			var err error
			converted[index], err = convertHTTPJSON(item, depth+1, items)
			if err != nil {
				return contract.Value{}, err
			}
		}
		return contract.ListValue(converted)
	case map[string]any:
		*items += len(value)
		if *items > contract.MaxValueItems {
			return contract.Value{}, errors.New("JSON exceeds item limit")
		}
		fields := make([]contract.Field, 0, len(value))
		keys := make([]string, 0, len(value))
		for key := range value {
			keys = append(keys, key)
		}
		sort.Strings(keys)
		for _, key := range keys {
			item := value[key]
			converted, err := convertHTTPJSON(item, depth+1, items)
			if err != nil {
				return contract.Value{}, err
			}
			fields = append(fields, contract.Field{Key: key, Value: converted})
		}
		return contract.ObjectValue(fields)
	default:
		return contract.Value{}, fmt.Errorf("unsupported JSON type %T", raw)
	}
}

func validateUniqueHTTPJSONKeys(payload []byte) error {
	decoder := json.NewDecoder(strings.NewReader(string(payload)))
	var consume func() error
	consume = func() error {
		token, err := decoder.Token()
		if err != nil {
			return err
		}
		delimiter, ok := token.(json.Delim)
		if !ok {
			return nil
		}
		switch delimiter {
		case '{':
			seen := map[string]struct{}{}
			for decoder.More() {
				keyToken, err := decoder.Token()
				if err != nil {
					return err
				}
				key, ok := keyToken.(string)
				if !ok {
					return errors.New("JSON object key is invalid")
				}
				if _, exists := seen[key]; exists {
					return errors.New("JSON object key is duplicated")
				}
				seen[key] = struct{}{}
				if err := consume(); err != nil {
					return err
				}
			}
			closing, err := decoder.Token()
			if err != nil || closing != json.Delim('}') {
				return errors.New("JSON object is invalid")
			}
		case '[':
			for decoder.More() {
				if err := consume(); err != nil {
					return err
				}
			}
			closing, err := decoder.Token()
			if err != nil || closing != json.Delim(']') {
				return errors.New("JSON array is invalid")
			}
		default:
			return errors.New("JSON delimiter is invalid")
		}
		return nil
	}
	if err := consume(); err != nil {
		return err
	}
	if _, err := decoder.Token(); !errors.Is(err, io.EOF) {
		return errors.New("JSON contains trailing data")
	}
	return nil
}
