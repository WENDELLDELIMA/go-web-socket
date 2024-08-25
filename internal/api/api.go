package api

import (
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"sync"

	"github.com/WENDELLDELIMA/go-web-socket/internal/store/pgstore"
	"github.com/google/uuid"
	"github.com/gorilla/websocket"
	"github.com/jackc/pgx/v5"

	"github.com/go-chi/chi/middleware"
	"github.com/go-chi/chi/v5"
	"github.com/go-chi/cors"
)

type apiHandler struct {
	q           *pgstore.Queries
	r           *chi.Mux
	uppgrader   websocket.Upgrader
	subscribers map[string]map[*websocket.Conn]context.CancelFunc
	mu          *sync.Mutex
}

func (h apiHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	h.r.ServeHTTP(w, r)
}
func jsonContentTypeMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		next.ServeHTTP(w, r)
	})
}
func NewHandler(q *pgstore.Queries) http.Handler {
	a := apiHandler{
		q:           q,
		uppgrader:   websocket.Upgrader{CheckOrigin: func(r *http.Request) bool { return true }},
		subscribers: make(map[string]map[*websocket.Conn]context.CancelFunc),
		mu:          &sync.Mutex{},
	}
	r := chi.NewRouter()
	r.Use(middleware.RequestID, middleware.Recoverer, middleware.Logger)
	// Basic CORS
	// for more ideas, see: https://developer.github.com/v3/#cross-origin-resource-sharing
	r.Use(cors.Handler(cors.Options{
		// AllowedOrigins:   []string{"https://foo.com"}, // Use this to allow specific origin hosts
		AllowedOrigins: []string{"https://*", "http://*"},
		// AllowOriginFunc:  func(r *http.Request, origin string) bool { return true },
		AllowedMethods:   []string{"GET", "POST", "PUT", "DELETE", "OPTIONS", "PATCH"},
		AllowedHeaders:   []string{"Accept", "Authorization", "Content-Type", "X-CSRF-Token"},
		ExposedHeaders:   []string{"Link"},
		AllowCredentials: false,
		MaxAge:           300, // Maximum value not ignored by any of major browsers
	}))
	r.Get("/subscribe/{room_id}", a.handleSubscribe)
	r.Route("/api", func(r chi.Router) {
		r.Use(jsonContentTypeMiddleware)
		r.Route("/rooms", func(r chi.Router) {
			r.Post("/", a.handleCreateRoom)
			r.Get("/", a.handleGetRooms)

			r.Route("/{room_id}/messages", func(r chi.Router) {
				r.Get("/", a.handleGetRoomMessages)
				r.Post("/", a.handleCreateRoomMessage)

			})
			r.Route("/{message_id}", func(r chi.Router) {
				r.Get("/", a.handleGetRoomMessage)
				r.Patch("/react", a.handleReactAndUnReactToMessage)
				r.Delete("/react", a.handleReactAndUnReactToMessage)

			})
		})
	})

	a.r = r

	return a
}

const (
	MessageKindMessageCreated = "message_created"
)

type MessageMessageCreated struct {
	ID      string
	Message string
}
type Message struct {
	Kind   string `json:"kind"`
	Value  any    `json:"value"`
	RoomId string `json:"-"`
}

func (h apiHandler) notifyClients(msg Message) {
	h.mu.Lock()
	defer h.mu.Unlock()

	subscribers, ok := h.subscribers[msg.RoomId]
	if !ok || len(subscribers) == 0 {
		return
	}
	for conn, cancel := range subscribers {
		if err := conn.WriteJSON(msg); err != nil {
			slog.Error("Failed to send message to client", "error", err)
			cancel()
		}
	}
}
func (h apiHandler) handleSubscribe(w http.ResponseWriter, r *http.Request) {
	rawRoomID := chi.URLParam(r, "room_id")
	roomID, err := uuid.Parse(rawRoomID)
	if err != nil {
		http.Error(w, "invalid room id", http.StatusBadRequest)
		return
	}
	_, err = h.q.GetRoom(r.Context(), roomID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			http.Error(w, "room not found", http.StatusBadRequest)
			return
		}
		http.Error(w, "something went wrong", http.StatusInternalServerError)
		return
	}
	c, err := h.uppgrader.Upgrade(w, r, nil)
	if err != nil {
		slog.Warn("failed to upgrade connection", "error", err)
		http.Error(w, "failed to upgrade to ws connection", http.StatusBadRequest)
		return
	}
	defer c.Close()
	ctx, cancel := context.WithCancel(r.Context())
	h.mu.Lock()
	if _, ok := h.subscribers[rawRoomID]; ok {
		h.subscribers[rawRoomID][c] = cancel
	} else {
		h.subscribers[rawRoomID] = make(map[*websocket.Conn]context.CancelFunc)
		h.subscribers[rawRoomID][c] = cancel
	}
	slog.Info("new client connected", "room_id", rawRoomID, "client_ip", r.RemoteAddr)
	h.mu.Unlock()
	<-ctx.Done()
	h.mu.Lock()
	delete(h.subscribers[rawRoomID], c)
	h.mu.Unlock()
}
func (h apiHandler) handleCreateRoom(w http.ResponseWriter, r *http.Request) {
	type _body struct {
		Theme string `json:"theme"`
	}
	var body _body
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		http.Error(w, "invalid Json", http.StatusBadRequest)
		return
	}
	roomID, err := h.q.InsertRoom(r.Context(), body.Theme)
	if err != nil {
		slog.Error("Failed to insert room", "error", err)
		http.Error(w, "something went wrong", http.StatusInternalServerError)
		return
	}
	type response struct {
		ID    string `json:"id"`
		Theme string `json:"theme"`
	}
	data, _ := json.Marshal(response{
		ID:    roomID.String(),
		Theme: body.Theme,
	})

	w.Write(data)

}

func (h apiHandler) handleCreateRoomMessage(w http.ResponseWriter, r *http.Request) {
	rawRoomID := chi.URLParam(r, "room_id")
	roomID, err := uuid.Parse(rawRoomID)
	if err != nil {
		http.Error(w, "invalid room id", http.StatusBadRequest)
		return
	}
	_, err = h.q.GetRoom(r.Context(), roomID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			http.Error(w, "room not found", http.StatusBadRequest)
			return
		}
		http.Error(w, "something went wrong", http.StatusInternalServerError)
		return
	}
	type _body struct {
		Message string `json:"message"`
	}
	var body _body
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		http.Error(w, "Invalid Json", http.StatusBadGateway)
		return
	}

	messageID, err := h.q.InsertMessage(r.Context(), pgstore.InsertMessageParams{RoomID: roomID, Message: body.Message})
	if err != nil {
		slog.Error("failed to insert message", "error", err)
		http.Error(w, "something went wrong", http.StatusInternalServerError)
		return
	}

	type response struct {
		ID      string `json:"id"`
		Message string `json:"Message"`
	}
	data, _ := json.Marshal(response{ID: messageID.String(), Message: body.Message})

	_, _ = w.Write(data)

	go h.notifyClients(Message{
		Kind:   MessageKindMessageCreated,
		RoomId: rawRoomID,
		Value: MessageMessageCreated{
			ID:      messageID.String(),
			Message: body.Message,
		},
	})
}

func (h apiHandler) handleGetRooms(w http.ResponseWriter, r *http.Request) {
	rooms, err := h.q.GetRooms(r.Context())
	if err != nil {
		http.Error(w, "Failed to get rooms", http.StatusInternalServerError)
		return
	}

	// converter a lista para json

	data, err := json.Marshal(rooms)

	if err != nil {
		http.Error(w, "Failed to encode response", http.StatusInternalServerError)
		return
	}

	w.Write(data)

}
func (h apiHandler) handleGetRoomMessages(w http.ResponseWriter, r *http.Request) {

	rawRoomID := chi.URLParam(r, "room_id")
	roomID, err := uuid.Parse(rawRoomID)
	if err != nil {
		slog.Error("Nao deu certo o id da sala")
		http.Error(w, "Failed to compare uuid", http.StatusBadRequest)
	}
	room, err := h.q.GetRoomMessages(r.Context(), roomID)
	if err != nil {
		http.Error(w, "Failed to get message Room", http.StatusInternalServerError)
		return
	}

	data, err := json.Marshal(room)
	if err != nil {
		http.Error(w, "Failed to encode response", http.StatusInternalServerError)
		return
	}

	w.Write(data)

}
func (h apiHandler) handleGetRoomMessage(w http.ResponseWriter, r *http.Request) {
	rawMessageID := chi.URLParam(r, "message_id")
	messageID, err := uuid.Parse(rawMessageID)
	if err != nil {
		http.Error(w, "Failed to compare uuid", http.StatusBadRequest)
		return
	}
	room, err := h.q.GetMessage(r.Context(), messageID)
	if err != nil {
		http.Error(w, "Failed to get  Room message", http.StatusInternalServerError)
		return
	}
	type response struct {
		ID string `json:"room_id"`
	}
	data, err := json.Marshal(response{ID: room.RoomID.String()})
	if err != nil {
		http.Error(w, "Failed to encode response", http.StatusInternalServerError)
		return
	}

	w.Write(data)
}

func (h apiHandler) handleReactAndUnReactToMessage(w http.ResponseWriter, r *http.Request) {
	rawMessageID := chi.URLParam(r, "message_id")
	messageID, err := uuid.Parse(rawMessageID)
	if err != nil {
		http.Error(w, "Failed to parse UUID", http.StatusBadRequest)
		return
	}

	var reactionCount int64
	switch r.Method {
	case http.MethodPatch:
		reactionCount, err = h.q.ReactToMessage(r.Context(), messageID)
	case http.MethodDelete:
		reactionCount, err = h.q.CountReactions(r.Context(), messageID)
		if err != nil {
			http.Error(w, "Failed to count reactions", http.StatusInternalServerError)
			return
		}
		if reactionCount == 0 {
			errorResponse := struct {
				Error string `json:"error"`
			}{
				Error: "Cannot decrease reactions below zero",
			}

			data, err := json.Marshal(errorResponse)
			if err != nil {
				http.Error(w, "Failed to encode error response", http.StatusInternalServerError)
				return
			}

			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusBadRequest) // Define o status HTTP como 400
			w.Write(data)
			return
		}
		reactionCount, err = h.q.RemoveReactToMessage(r.Context(), messageID)
	default:
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	if err != nil {
		http.Error(w, "Failed to process request", http.StatusInternalServerError)
		return
	}

	response := struct {
		ReactionCount int64 `json:"reaction_count"`
	}{
		ReactionCount: reactionCount,
	}

	data, err := json.Marshal(response)
	if err != nil {
		http.Error(w, "Failed to encode response", http.StatusInternalServerError)
		return
	}

	w.Write(data)
}
