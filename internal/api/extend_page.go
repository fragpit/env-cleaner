package api

import (
	_ "embed"
	"html/template"
	"log/slog"
	"net/http"

	"github.com/xhit/go-str2duration/v2"
)

//go:embed static/extend.html
var extendHTML string

//go:embed static/extend.css
var extendCSS string

//go:embed static/extend.js
var extendJS string

type extendPageData struct {
	EnvID     string
	Name      string
	Type      string
	Owner     string
	DeleteAt  string
	Token     string
	PeriodMin string
	PeriodMid string
	PeriodMax string
}

type ExtendPageHandler struct {
	service           EnvironmentService
	staleThreshold    string
	maxExtendDuration string
	tmpl              *template.Template
}

func NewExtendPageHandler(
	svc EnvironmentService,
	staleThreshold string,
	maxExtendDuration string,
) *ExtendPageHandler {
	tmpl := template.Must(
		template.New("extend").Parse(extendHTML),
	)
	return &ExtendPageHandler{
		service:           svc,
		staleThreshold:    staleThreshold,
		maxExtendDuration: maxExtendDuration,
		tmpl:              tmpl,
	}
}

func (h *ExtendPageHandler) ServePage(
	w http.ResponseWriter,
	r *http.Request,
) {
	envID := r.URL.Query().Get("env_id")
	token := r.URL.Query().Get("token")

	if envID == "" || token == "" {
		sendErrorResponse(
			w, http.StatusBadRequest,
			"missing env_id or token",
		)
		return
	}

	env, err := h.service.GetEnvironmentForExtend(
		r.Context(), envID, token,
	)
	if err != nil {
		handleServiceError(w, err, envID)
		return
	}

	periods, err := calcExtendPeriods(
		h.staleThreshold, h.maxExtendDuration,
	)
	if err != nil {
		slog.Error("error calculating extend periods",
			slog.Any("error", err),
		)
		sendErrorResponse(
			w, http.StatusInternalServerError,
			"internal server error",
		)
		return
	}

	data := extendPageData{
		EnvID:     env.EnvID,
		Name:      env.DisplayName(),
		Type:      env.Type,
		Owner:     env.Owner,
		DeleteAt:  env.DeleteAt,
		Token:     token,
		PeriodMin: periods["min"],
		PeriodMid: periods["mid"],
		PeriodMax: periods["max"],
	}

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	if err := h.tmpl.Execute(w, data); err != nil {
		slog.Error("error rendering template",
			slog.Any("error", err),
		)
	}
}

func (h *ExtendPageHandler) ServeCSS(
	w http.ResponseWriter,
	r *http.Request,
) {
	w.Header().Set("Content-Type", "text/css; charset=utf-8")
	w.WriteHeader(http.StatusOK)
	if _, err := w.Write([]byte(extendCSS)); err != nil {
		slog.Error("error serving CSS",
			slog.Any("error", err),
		)
	}
}

func (h *ExtendPageHandler) ServeJS(
	w http.ResponseWriter,
	r *http.Request,
) {
	w.Header().Set(
		"Content-Type",
		"application/javascript; charset=utf-8",
	)
	w.WriteHeader(http.StatusOK)
	if _, err := w.Write([]byte(extendJS)); err != nil {
		slog.Error("error serving JS",
			slog.Any("error", err),
		)
	}
}

func calcExtendPeriods(
	staleThreshold, maxExtendDuration string,
) (map[string]string, error) {
	maxDuration, err := str2duration.ParseDuration(
		maxExtendDuration,
	)
	if err != nil {
		return nil, err
	}

	midDuration := maxDuration / 2

	return map[string]string{
		"min": staleThreshold,
		"mid": str2duration.String(midDuration),
		"max": maxExtendDuration,
	}, nil
}
