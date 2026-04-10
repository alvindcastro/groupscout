package aviation

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestParseNOTAMs(t *testing.T) {
	fixture := `
	<div class="notam">
		<pre>CYVR GND STOP ALL ACFT DUE TO WEATHER AT CYVR</pre>
	</div>
	`
	isGroundStop, err := parseNOTAMs(strings.NewReader(fixture))
	if err != nil {
		t.Fatalf("failed to parse: %v", err)
	}

	if !isGroundStop {
		t.Errorf("expected GroundStop to be true")
	}
}

func TestParseNOTAMs_Normal(t *testing.T) {
	fixture := `
	<div class="notam">
		<pre>CYVR RWY 08L/26R CLSD DUE TO MAINT</pre>
	</div>
	`
	isGroundStop, err := parseNOTAMs(strings.NewReader(fixture))
	if err != nil {
		t.Fatalf("failed to parse: %v", err)
	}

	if isGroundStop {
		t.Errorf("expected GroundStop to be false")
	}
}

func TestNavCanadaClient_FetchGroundStop(t *testing.T) {
	fixture := `CYVR GND STOP`
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(fixture))
	}))
	defer server.Close()

	client := &NavCanadaClient{
		client: server.Client(),
		url:    server.URL,
	}

	isGroundStop, err := client.FetchGroundStop(context.Background(), "CYVR")
	if err != nil {
		t.Fatalf("FetchGroundStop failed: %v", err)
	}

	if !isGroundStop {
		t.Errorf("expected GroundStop to be true")
	}
}
