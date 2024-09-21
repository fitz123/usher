package main

import (
	"encoding/json"
	"net/http"
	"sync"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
)

const (
	expirationDuration = 10 * time.Second // Ticket expiration time
)

type TicketInfo struct {
	Timestamp time.Time `json:"updated_at"`
	Timer     *time.Timer
}

type Server struct {
	tickets            sync.Map // map[ticketID]TicketInfo
	expirationDuration time.Duration
}

func main() {
	s := &Server{
		expirationDuration: expirationDuration,
	}

	r := chi.NewRouter()
	r.Use(middleware.Logger)

	r.Get("/api/v1/ticket/validate/{ticket_id}/{user_id}", s.validateHandler)
	r.Get("/api/v1/ticket/list/{space}", s.listHandler)

	http.ListenAndServe(":8001", r)
}

// validateHandler handles ticket validation requests.
func (s *Server) validateHandler(w http.ResponseWriter, r *http.Request) {
	ticketID := chi.URLParam(r, "ticket_id")
	if ticketID == "" {
		http.Error(w, "Missing ticket ID", http.StatusBadRequest)
		return
	}

	currentTime := time.Now()

	val, loaded := s.tickets.Load(ticketID)
	if !loaded {
		// Ticket does not exist, create a new one with a timer
		timer := time.AfterFunc(s.expirationDuration, func() {
			s.tickets.Delete(ticketID)
		})

		ticketInfo := TicketInfo{
			Timestamp: currentTime,
			Timer:     timer,
		}

		s.tickets.Store(ticketID, ticketInfo)
	} else {
		// Ticket exists, reset the timer and update info
		existingTicketInfo := val.(TicketInfo)
		existingTicketInfo.Timer.Reset(s.expirationDuration)

		ticketInfo := TicketInfo{
			Timestamp: currentTime,
			Timer:     existingTicketInfo.Timer,
		}

		s.tickets.Store(ticketID, ticketInfo)
	}

	w.WriteHeader(http.StatusOK)
	w.Write([]byte("OK"))
}

// listHandler returns the list of active tickets.
func (s *Server) listHandler(w http.ResponseWriter, r *http.Request) {
	ticketsList := []map[string]string{}

	s.tickets.Range(func(key, value interface{}) bool {
		ticketID := key.(string)
		ticketInfo := value.(TicketInfo)

		ticketsList = append(ticketsList, map[string]string{
			"ticket_id":  ticketID,
			"updated_at": ticketInfo.Timestamp.Format("2006-01-02 15:04:05"),
		})
		return true
	})

	response, err := json.Marshal(ticketsList)
	if err != nil {
		http.Error(w, "Error generating response", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.Write(response)
}
