package config

import (
	"net/http"
	"reflect"
	"testing"
	"time"

	"github.com/magiconair/properties"
	"github.com/pascaldekloe/goe/verify"
)

func TestFromProperties(t *testing.T) {
	in := `
proxy.cs = cs=name;type=path;cert=foo;clientca=bar;refresh=99s;hdr=a: b;caupgcn=furb
proxy.addr = :1234
proxy.localip = 4.4.4.4
proxy.strategy = rr
proxy.matcher = prefix
proxy.noroutestatus = 929
proxy.shutdownwait = 500ms
proxy.responseheadertimeout = 3s
proxy.keepalivetimeout = 4s
proxy.dialtimeout = 60s
proxy.readtimeout = 5s
proxy.writetimeout = 10s
proxy.flushinterval = 15s
proxy.maxconn = 666
proxy.header.clientip = clientip
proxy.header.tls = tls
proxy.header.tls.value = tls-true
registry.backend = something
registry.file.path = /foo/bar
registry.static.routes = route add svc / http://127.0.0.1:6666/
registry.consul.addr = https://1.2.3.4:5678
registry.consul.token = consul-token
registry.consul.kvpath = /some/path
registry.consul.tagprefix = p-
registry.consul.register.enabled = false
registry.consul.register.addr = 6.6.6.6:7777
registry.consul.register.name = fab
registry.consul.register.tags = a, b, c ,
registry.consul.register.checkInterval = 5s
registry.consul.register.checkTimeout = 10s
registry.consul.service.status = a,b
metrics.target = graphite
metrics.prefix = someprefix
metrics.interval = 5s
metrics.graphite.addr = 5.6.7.8:9999
runtime.gogc = 666
runtime.gomaxprocs = 12
ui.addr = 7.8.9.0:1234
ui.color = fonzy
ui.title = fabfab
aws.apigw.cert.cn = furb
`
	out := &Config{
		ListenerValue:    []string{":1234"},
		CertSourcesValue: []map[string]string{{"cs": "name", "type": "path", "cert": "foo", "clientca": "bar", "refresh": "99s", "hdr": "a: b", "caupgcn": "furb"}},
		CertSources: map[string]CertSource{
			"name": CertSource{
				Name:         "name",
				Type:         "path",
				CertPath:     "foo",
				ClientCAPath: "bar",
				CAUpgradeCN:  "furb",
				Refresh:      99 * time.Second,
				Header:       http.Header{"A": []string{"b"}},
			},
		},
		Proxy: Proxy{
			MaxConn:               666,
			LocalIP:               "4.4.4.4",
			Strategy:              "rr",
			Matcher:               "prefix",
			NoRouteStatus:         929,
			ShutdownWait:          500 * time.Millisecond,
			DialTimeout:           60 * time.Second,
			ResponseHeaderTimeout: 3 * time.Second,
			KeepAliveTimeout:      4 * time.Second,
			ReadTimeout:           5 * time.Second,
			WriteTimeout:          10 * time.Second,
			FlushInterval:         15 * time.Second,
			ClientIPHeader:        "clientip",
			TLSHeader:             "tls",
			TLSHeaderValue:        "tls-true",
		},
		Registry: Registry{
			Backend: "something",
			File: File{
				Path: "/foo/bar",
			},
			Static: Static{
				Routes: "route add svc / http://127.0.0.1:6666/",
			},
			Consul: Consul{
				Addr:          "1.2.3.4:5678",
				Scheme:        "https",
				Token:         "consul-token",
				KVPath:        "/some/path",
				TagPrefix:     "p-",
				Register:      false,
				ServiceAddr:   "6.6.6.6:7777",
				ServiceName:   "fab",
				ServiceTags:   []string{"a", "b", "c"},
				ServiceStatus: []string{"a", "b"},
				CheckInterval: 5 * time.Second,
				CheckTimeout:  10 * time.Second,
			},
		},
		Listen: []Listen{
			{
				Addr:         ":1234",
				Scheme:       "http",
				ReadTimeout:  5 * time.Second,
				WriteTimeout: 10 * time.Second,
			},
		},
		Metrics: Metrics{
			Target:       "graphite",
			Prefix:       "someprefix",
			Interval:     5 * time.Second,
			GraphiteAddr: "5.6.7.8:9999",
		},
		Runtime: Runtime{
			GOGC:       666,
			GOMAXPROCS: 12,
		},
		UI: UI{
			Addr:  "7.8.9.0:1234",
			Color: "fonzy",
			Title: "fabfab",
		},
	}

	p, err := properties.Load([]byte(in), properties.UTF8)
	if err != nil {
		t.Fatalf("got %v want nil", err)
	}

	cfg, err := load(p)
	if err != nil {
		t.Fatalf("got %v want nil", err)
	}

	got, want := cfg, out
	verify.Values(t, "cfg", got, want)
}

func TestParseScheme(t *testing.T) {
	tests := []struct {
		in           string
		scheme, addr string
	}{
		{"foo:bar", "http", "foo:bar"},
		{"http://foo:bar", "http", "foo:bar"},
		{"https://foo:bar", "https", "foo:bar"},
		{"HTTPS://FOO:bar", "https", "foo:bar"},
	}

	for i, tt := range tests {
		scheme, addr := parseScheme(tt.in)
		if got, want := scheme, tt.scheme; got != want {
			t.Errorf("%d: got %v want %v", i, got, want)
		}
		if got, want := addr, tt.addr; got != want {
			t.Errorf("%d: got %v want %v", i, got, want)
		}
	}
}

func TestParseListen(t *testing.T) {
	cs := map[string]CertSource{
		"name": CertSource{Type: "foo"},
	}

	tests := []struct {
		in  string
		out Listen
		err string
	}{
		{
			"",
			Listen{},
			"",
		},
		{
			":123",
			Listen{Addr: ":123", Scheme: "http"},
			"",
		},
		{
			":123;rt=5s;wt=5s",
			Listen{Addr: ":123", Scheme: "http", ReadTimeout: 5 * time.Second, WriteTimeout: 5 * time.Second},
			"",
		},
		{
			":123;pathA;pathB;pathC",
			Listen{
				Addr:   ":123",
				Scheme: "https",
				CertSource: CertSource{
					Type:         "file",
					CertPath:     "pathA",
					KeyPath:      "pathB",
					ClientCAPath: "pathC",
				},
			},
			"",
		},
		{
			":123;cs=name",
			Listen{
				Addr:   ":123",
				Scheme: "https",
				CertSource: CertSource{
					Type: "foo",
				},
			},
			"",
		},
	}

	for i, tt := range tests {
		l, err := parseListen(tt.in, cs, time.Duration(0), time.Duration(0))
		if got, want := err, tt.err; (got != nil || want != "") && got.Error() != want {
			t.Errorf("%d: got %+v want %+v", i, got, want)
		}
		if got, want := l, tt.out; !reflect.DeepEqual(got, want) {
			t.Errorf("%d: got %+v want %+v", i, got, want)
		}
	}
}

func TestParseCfg(t *testing.T) {
	tests := []struct {
		args []string
		i    int
		path string
		err  error
	}{
		// edge cases
		{nil, 0, ``, nil},
		{[]string{`-abc`}, 0, "", nil},
		{[]string{`-cfg`}, 1, "", nil},
		{[]string{`-cfg`}, 5, "", nil},

		// errors
		{[]string{`-cfg`}, 0, "", errInvalidConfig},
		{[]string{`-cfg=''`}, 0, "", errInvalidConfig},
		{[]string{`-cfg=""`}, 0, "", errInvalidConfig},
		{[]string{`-cfg=`}, 0, "", errInvalidConfig},

		// happy flow
		{[]string{`-cfg`, `foo`}, 0, "foo", nil},
		{[]string{`-cfg=foo`}, 0, "foo", nil},
		{[]string{`-cfg='foo'`}, 0, "foo", nil},
		{[]string{`-cfg="foo"`}, 0, "foo", nil},
		{[]string{`-cfg='"foo"'`}, 0, `"foo"`, nil},
		{[]string{`-cfg="'foo'"`}, 0, `'foo'`, nil},
	}

	for i, tt := range tests {
		p, err := parseCfg(tt.args, tt.i)
		if got, want := err, tt.err; got != want {
			t.Fatalf("%d: got %v want %v", i, got, want)
		}
		if got, want := p, tt.path; got != want {
			t.Errorf("%d: got %v want %v", i, got, want)
		}
	}
}
