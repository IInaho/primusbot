package httpapi

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"

	controlruntime "nekocode/runtime"
)

type Runtime interface {
	controlruntime.Control
	controlruntime.Query
}

type Server struct {
	rt Runtime
}

func New(rt Runtime) *Server {
	return &Server{rt: rt}
}

func (s *Server) Handler() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /health", s.handleHealth)
	mux.HandleFunc("GET /runs", s.handleRuns)
	mux.HandleFunc("GET /runs/current", s.handleCurrentRun)
	mux.HandleFunc("GET /connect", s.handleConnect)
	mux.HandleFunc("POST /connect/{name}", s.handleConnectCommand)
	mux.HandleFunc("POST /disconnect/{name}", s.handleDisconnectCommand)
	mux.HandleFunc("GET /model", s.handleModel)
	mux.HandleFunc("GET /commands", s.handleCommands)
	mux.HandleFunc("GET /stats", s.handleStats)
	mux.HandleFunc("GET /context", s.handleContext)
	mux.HandleFunc("GET /memory", s.handleMemory)
	mux.HandleFunc("GET /sessions", s.handleSessions)
	mux.HandleFunc("GET /sessions/current/messages", s.handleSessionMessages)
	mux.HandleFunc("POST /input", s.handleInput)
	mux.HandleFunc("GET /events", s.handleEvents)
	mux.HandleFunc("GET /runs/{runID}", s.handleRun)
	mux.HandleFunc("GET /runs/{runID}/artifacts", s.handleArtifacts)
	mux.HandleFunc("POST /runs/{runID}/abort", s.handleAbort)
	mux.HandleFunc("POST /approvals/{approvalID}", s.handleApproval)
	mux.HandleFunc("POST /questions/{questionID}", s.handleQuestion)
	return mux
}

type submitRequest struct {
	Text      string                    `json:"text"`
	Kind      controlruntime.InputKind  `json:"kind,omitempty"`
	Source    *controlruntime.SourceRef `json:"source,omitempty"`
	Sender    *controlruntime.SenderRef `json:"sender,omitempty"`
	SessionID string                    `json:"session_id,omitempty"`
	ReplyTo   string                    `json:"reply_to,omitempty"`
}

type submitResponse struct {
	RunID controlruntime.RunID `json:"run_id"`
}

type approvalRequest struct {
	Allowed             bool `json:"allowed"`
	Remember            bool `json:"remember,omitempty"`
	AllowWithPermission bool `json:"allow_with_permission,omitempty"`
}

type questionRequest struct {
	Answers  [][]string `json:"answers,omitempty"`
	Rejected bool       `json:"rejected,omitempty"`
}

type connectRequest struct {
	Args []string `json:"args,omitempty"`
}

type eventHistoryRuntime interface {
	EventHistory(filter controlruntime.EventFilter) []controlruntime.Event
}

type replayRuntime interface {
	SubscribeReplay(ctx context.Context, filter controlruntime.EventFilter) (<-chan controlruntime.Event, error)
}

func (s *Server) handleHealth(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, map[string]any{"ok": true})
}

func (s *Server) handleRuns(w http.ResponseWriter, r *http.Request) {
	limit := parseLimit(r.URL.Query().Get("limit"), 50)
	writeJSON(w, http.StatusOK, map[string]any{"runs": s.rt.ListRunViews(limit)})
}

func (s *Server) handleCurrentRun(w http.ResponseWriter, _ *http.Request) {
	run, ok := s.rt.CurrentRunView()
	if !ok {
		writeError(w, http.StatusNotFound, "current run not found")
		return
	}
	writeJSON(w, http.StatusOK, run)
}

func (s *Server) handleConnect(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, s.rt.ConnectView())
}

func (s *Server) handleConnectCommand(w http.ResponseWriter, r *http.Request) {
	var req connectRequest
	if err := decodeOptionalJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	resp, err := s.rt.Connect(r.Context(), r.PathValue("name"), req.Args)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"message": resp})
}

func (s *Server) handleDisconnectCommand(w http.ResponseWriter, r *http.Request) {
	resp, err := s.rt.Disconnect(r.PathValue("name"))
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"message": resp})
}

func (s *Server) handleModel(w http.ResponseWriter, _ *http.Request) {
	provider, model := s.rt.ProviderModel()
	writeJSON(w, http.StatusOK, map[string]any{
		"provider": provider,
		"model":    model,
	})
}

func (s *Server) handleCommands(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, map[string]any{"commands": s.rt.CommandNames()})
}

func (s *Server) handleStats(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, s.rt.Stats())
}

func (s *Server) handleContext(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, s.rt.ContextSnapshot())
}

func (s *Server) handleMemory(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, s.rt.MemoryView(controlruntime.MemoryScope(r.URL.Query().Get("scope"))))
}

func (s *Server) handleSessions(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, map[string]any{"sessions": s.rt.ListSessions()})
}

func (s *Server) handleSessionMessages(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, map[string]any{"messages": s.rt.SessionMessages()})
}

func (s *Server) handleRun(w http.ResponseWriter, r *http.Request) {
	runID := controlruntime.RunID(r.PathValue("runID"))
	run, ok := s.rt.RunView(runID)
	if !ok {
		writeError(w, http.StatusNotFound, "run not found")
		return
	}
	writeJSON(w, http.StatusOK, run)
}

func (s *Server) handleArtifacts(w http.ResponseWriter, r *http.Request) {
	runID := controlruntime.RunID(r.PathValue("runID"))
	artifact, ok := s.rt.ArtifactView(runID)
	if !ok {
		writeError(w, http.StatusNotFound, "run artifacts not found")
		return
	}
	writeJSON(w, http.StatusOK, artifact)
}

func (s *Server) handleInput(w http.ResponseWriter, r *http.Request) {
	var req submitRequest
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	if strings.TrimSpace(req.Text) == "" {
		writeError(w, http.StatusBadRequest, "text is required")
		return
	}
	source := controlruntime.SourceRef{Kind: "http"}
	if req.Source != nil {
		source = *req.Source
		if source.Kind == "" {
			source.Kind = "http"
		}
	}
	var sender controlruntime.SenderRef
	if req.Sender != nil {
		sender = *req.Sender
	}
	runID, err := s.rt.Submit(r.Context(), controlruntime.Input{
		Kind:      req.Kind,
		Source:    source,
		Sender:    sender,
		Text:      req.Text,
		SessionID: req.SessionID,
		ReplyTo:   req.ReplyTo,
	})
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	writeJSON(w, http.StatusAccepted, submitResponse{RunID: runID})
}

func (s *Server) handleAbort(w http.ResponseWriter, r *http.Request) {
	runID := controlruntime.RunID(r.PathValue("runID"))
	if err := s.rt.Abort(r.Context(), runID); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"ok": true})
}

func (s *Server) handleApproval(w http.ResponseWriter, r *http.Request) {
	var req approvalRequest
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	err := s.rt.Approve(r.Context(), r.PathValue("approvalID"), controlruntime.ApprovalDecision{
		Allowed:             req.Allowed,
		Remember:            req.Remember,
		AllowWithPermission: req.AllowWithPermission,
	})
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"ok": true})
}

func (s *Server) handleQuestion(w http.ResponseWriter, r *http.Request) {
	var req questionRequest
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	err := s.rt.Answer(r.Context(), r.PathValue("questionID"), controlruntime.QuestionReply{
		Answers:  req.Answers,
		Rejected: req.Rejected,
	})
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"ok": true})
}

func (s *Server) handleEvents(w http.ResponseWriter, r *http.Request) {
	flusher, ok := w.(http.Flusher)
	if !ok {
		writeError(w, http.StatusInternalServerError, "streaming is not supported")
		return
	}
	filter := controlruntime.EventFilter{
		RunID: controlruntime.RunID(r.URL.Query().Get("run_id")),
	}
	var events <-chan controlruntime.Event
	var err error
	if r.URL.Query().Get("replay") == "1" {
		if replayRT, ok := s.rt.(replayRuntime); ok {
			events, err = replayRT.SubscribeReplay(r.Context(), filter)
		} else {
			events, err = s.rt.Subscribe(r.Context(), filter)
		}
	} else {
		events, err = s.rt.Subscribe(r.Context(), filter)
	}
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	w.WriteHeader(http.StatusOK)
	flusher.Flush()

	for {
		select {
		case <-r.Context().Done():
			return
		case ev, ok := <-events:
			if !ok {
				return
			}
			if err := writeSSE(w, ev); err != nil {
				return
			}
			flusher.Flush()
		}
	}
}

func decodeJSON(r *http.Request, out any) error {
	defer r.Body.Close()
	if err := decodeSingleJSON(r.Body, out); err != nil {
		return fmt.Errorf("invalid json: %w", err)
	}
	return nil
}

func decodeOptionalJSON(r *http.Request, out any) error {
	if r.Body == nil || r.ContentLength == 0 {
		return nil
	}
	defer r.Body.Close()
	if err := decodeSingleJSON(r.Body, out); err != nil {
		if errors.Is(err, io.EOF) {
			return nil
		}
		return fmt.Errorf("invalid json: %w", err)
	}
	return nil
}

func decodeSingleJSON(body io.Reader, out any) error {
	dec := json.NewDecoder(body)
	dec.DisallowUnknownFields()
	if err := dec.Decode(out); err != nil {
		return err
	}
	var extra any
	if err := dec.Decode(&extra); err == nil {
		return errors.New("multiple json values")
	} else if !errors.Is(err, io.EOF) {
		return err
	}
	return nil
}

func writeJSON(w http.ResponseWriter, status int, value any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(value)
}

func writeError(w http.ResponseWriter, status int, message string) {
	writeJSON(w, status, map[string]any{"error": message})
}

func writeSSE(w http.ResponseWriter, ev controlruntime.Event) error {
	data, err := json.Marshal(ev)
	if err != nil {
		return err
	}
	if ev.ID != "" {
		if _, err := fmt.Fprintf(w, "id: %s\n", ev.ID); err != nil {
			return err
		}
	}
	if _, err := fmt.Fprintf(w, "event: %s\n", ev.Type); err != nil {
		return err
	}
	_, err = fmt.Fprintf(w, "data: %s\n\n", data)
	return err
}

func parseLimit(raw string, fallback int) int {
	if raw == "" {
		return fallback
	}
	var n int
	if _, err := fmt.Sscanf(raw, "%d", &n); err != nil || n <= 0 {
		return fallback
	}
	if n > 200 {
		return 200
	}
	return n
}
