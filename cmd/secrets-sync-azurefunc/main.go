// Command secrets-sync-azurefunc is the Azure Functions entrypoint, implemented
// as an Azure Functions custom handler. The Functions host invokes this process
// over HTTP; the handler decodes the trigger payload into a serverless Request,
// runs the pipeline via pkg/serverless, and returns the Response as JSON.
//
// It is a thin adapter — all execution logic lives in pkg/serverless, shared
// with the AWS Lambda entrypoint.
package main

import (
	"encoding/json"
	"fmt"
	"net/http"
	"os"

	"github.com/jbcom/secrets-sync/pkg/serverless"
)

// invocationRequest is the body the Azure Functions host POSTs to a custom
// handler: trigger inputs are delivered under Data keyed by binding name, plus
// invocation metadata. We accept the serverless Request either as the whole
// body (direct invocation) or under Data["req"]/Data["body"] for an HTTP
// trigger.
type invocationRequest struct {
	Data map[string]json.RawMessage `json:"Data"`
}

func main() {
	port := os.Getenv("FUNCTIONS_CUSTOMHANDLER_PORT")
	if port == "" {
		port = "8080"
	}
	mux := http.NewServeMux()
	// The function name path is configured in function.json; "sync" is the
	// conventional name used by this handler.
	mux.HandleFunc("/sync", handleInvocation)
	mux.HandleFunc("/", handleInvocation)

	server := &http.Server{Addr: ":" + port, Handler: mux}
	if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		fmt.Fprintf(os.Stderr, "azurefunc: server error: %v\n", err)
		os.Exit(1)
	}
}

func handleInvocation(w http.ResponseWriter, r *http.Request) {
	req, err := decodeRequest(r)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, serverless.Response{Success: false, ErrorMessage: err.Error()})
		return
	}

	resp := serverless.Handle(r.Context(), req)
	status := http.StatusOK
	if !resp.Success {
		status = http.StatusInternalServerError
	}
	writeJSON(w, status, resp)
}

// decodeRequest extracts a serverless.Request from the Azure Functions custom-
// handler invocation envelope, falling back to treating the raw body as the
// Request for direct/manual invocations.
func decodeRequest(r *http.Request) (serverless.Request, error) {
	var raw json.RawMessage
	dec := json.NewDecoder(r.Body)
	if err := dec.Decode(&raw); err != nil {
		return serverless.Request{}, fmt.Errorf("decode invocation body: %w", err)
	}

	// Try the custom-handler envelope first.
	var inv invocationRequest
	if err := json.Unmarshal(raw, &inv); err == nil && len(inv.Data) > 0 {
		for _, key := range []string{"req", "body", "request"} {
			if payload, ok := inv.Data[key]; ok {
				req, perr := decodeServerlessRequest(payload)
				if perr == nil {
					return req, nil
				}
			}
		}
	}

	// Fall back to the raw body being the Request itself.
	return decodeServerlessRequest(raw)
}

// decodeServerlessRequest decodes a serverless.Request from JSON that may itself
// be a JSON string (HTTP-trigger bodies are delivered as a string).
func decodeServerlessRequest(payload json.RawMessage) (serverless.Request, error) {
	var req serverless.Request
	if err := json.Unmarshal(payload, &req); err == nil {
		return req, nil
	}
	// The payload might be a JSON-encoded string containing the request JSON.
	var asString string
	if err := json.Unmarshal(payload, &asString); err == nil {
		if err := json.Unmarshal([]byte(asString), &req); err == nil {
			return req, nil
		}
	}
	return serverless.Request{}, fmt.Errorf("payload is not a serverless request")
}

func writeJSON(w http.ResponseWriter, status int, resp serverless.Response) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_, _ = w.Write([]byte(serverless.MarshalResponse(resp)))
}
