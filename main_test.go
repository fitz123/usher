package main

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strconv"
	"sync"
	"testing"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/stretchr/testify/assert"
)

func TestServer_ValidateHandler_NewTicket(t *testing.T) {
	s := &Server{
		expirationDuration: 100 * time.Millisecond, // Reduced duration for faster tests
	}

	ticketID := "ticket123"
	userID := "user456"
	req := httptest.NewRequest("GET", "/api/v1/ticket/validate/"+ticketID+"/"+userID, nil)

	rctx := chi.NewRouteContext()
	rctx.URLParams.Add("ticket_id", ticketID)
	rctx.URLParams.Add("user_id", userID)
	req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))

	rr := httptest.NewRecorder()
	s.validateHandler(rr, req)

	assert.Equal(t, http.StatusOK, rr.Code)

	val, ok := s.tickets.Load(ticketID)
	assert.True(t, ok, "Ticket should be in the tickets map")
	assert.NotNil(t, val, "Ticket info should not be nil")
	ticketInfo := val.(TicketInfo)
	assert.WithinDuration(t, time.Now(), ticketInfo.Timestamp, time.Second)
}

func TestServer_ValidateHandler_UpdateTicket(t *testing.T) {
	s := &Server{
		expirationDuration: 200 * time.Millisecond, // Reduced duration
	}

	ticketID := "ticket123"
	userID := "user456"

	// First validation
	req1 := httptest.NewRequest("GET", "/api/v1/ticket/validate/"+ticketID+"/"+userID, nil)
	rctx1 := chi.NewRouteContext()
	rctx1.URLParams.Add("ticket_id", ticketID)
	rctx1.URLParams.Add("user_id", userID)
	req1 = req1.WithContext(context.WithValue(req1.Context(), chi.RouteCtxKey, rctx1))

	rr1 := httptest.NewRecorder()
	s.validateHandler(rr1, req1)
	assert.Equal(t, http.StatusOK, rr1.Code)

	val1, ok1 := s.tickets.Load(ticketID)
	assert.True(t, ok1)
	ticketInfo1 := val1.(TicketInfo)
	timestamp1 := ticketInfo1.Timestamp

	// Wait for some time
	time.Sleep(50 * time.Millisecond)

	// Second validation
	req2 := httptest.NewRequest("GET", "/api/v1/ticket/validate/"+ticketID+"/"+userID, nil)
	rctx2 := chi.NewRouteContext()
	rctx2.URLParams.Add("ticket_id", ticketID)
	rctx2.URLParams.Add("user_id", userID)
	req2 = req2.WithContext(context.WithValue(req2.Context(), chi.RouteCtxKey, rctx2))

	rr2 := httptest.NewRecorder()
	s.validateHandler(rr2, req2)
	assert.Equal(t, http.StatusOK, rr2.Code)

	val2, ok2 := s.tickets.Load(ticketID)
	assert.True(t, ok2)
	ticketInfo2 := val2.(TicketInfo)
	timestamp2 := ticketInfo2.Timestamp

	// Check that the timestamp has been updated
	assert.True(t, timestamp2.After(timestamp1))

	// Check that the timer has been reset by ensuring the ticket is still present after (expirationDuration - 50ms)
	time.Sleep(150 * time.Millisecond)

	// The ticket should still be in the tickets map
	_, ok3 := s.tickets.Load(ticketID)
	assert.True(t, ok3, "Ticket should still be in the tickets map")
}

func TestServer_TicketExpiration(t *testing.T) {
	s := &Server{
		expirationDuration: 100 * time.Millisecond, // Reduced duration
	}

	ticketID := "ticket123"
	userID := "user456"

	// Validate ticket
	req := httptest.NewRequest("GET", "/api/v1/ticket/validate/"+ticketID+"/"+userID, nil)
	rctx := chi.NewRouteContext()
	rctx.URLParams.Add("ticket_id", ticketID)
	rctx.URLParams.Add("user_id", userID)
	req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))

	rr := httptest.NewRecorder()
	s.validateHandler(rr, req)
	assert.Equal(t, http.StatusOK, rr.Code)

	_, ok := s.tickets.Load(ticketID)
	assert.True(t, ok)

	// Wait for expiration duration plus a small buffer
	time.Sleep(150 * time.Millisecond)

	// Check that the ticket has been removed
	_, ok = s.tickets.Load(ticketID)
	assert.False(t, ok, "Ticket should have expired and been removed from tickets map")
}

func TestServer_ListHandler(t *testing.T) {
	s := &Server{
		expirationDuration: 100 * time.Millisecond, // Reduced duration
	}

	// Validate multiple tickets
	ticketIDs := []string{"ticket1", "ticket2", "ticket3"}
	userID := "user456"

	for _, ticketID := range ticketIDs {
		req := httptest.NewRequest("GET", "/api/v1/ticket/validate/"+ticketID+"/"+userID, nil)
		rctx := chi.NewRouteContext()
		rctx.URLParams.Add("ticket_id", ticketID)
		rctx.URLParams.Add("user_id", userID)
		req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))

		rr := httptest.NewRecorder()
		s.validateHandler(rr, req)
		assert.Equal(t, http.StatusOK, rr.Code)
	}

	// Call listHandler
	reqList := httptest.NewRequest("GET", "/api/v1/ticket/list/anyspace", nil)
	rctxList := chi.NewRouteContext()
	rctxList.URLParams.Add("space", "anyspace")
	reqList = reqList.WithContext(context.WithValue(reqList.Context(), chi.RouteCtxKey, rctxList))

	rrList := httptest.NewRecorder()
	s.listHandler(rrList, reqList)

	assert.Equal(t, http.StatusOK, rrList.Code)
	var ticketsList []map[string]string
	err := json.Unmarshal(rrList.Body.Bytes(), &ticketsList)
	assert.NoError(t, err)
	assert.Len(t, ticketsList, len(ticketIDs))

	// Verify that all ticket IDs are present
	returnedTicketIDs := make(map[string]bool)
	for _, ticket := range ticketsList {
		returnedTicketIDs[ticket["ticket_id"]] = true
	}

	for _, ticketID := range ticketIDs {
		assert.True(t, returnedTicketIDs[ticketID], "Ticket ID %s should be in the list", ticketID)
	}
}

func TestServer_HighRPS(t *testing.T) {
	s := &Server{
		expirationDuration: 100 * time.Millisecond, // Reduced duration
	}

	numRequests := 1000
	ticketID := "ticket_high_rps"
	userID := "user456"

	var wg sync.WaitGroup
	wg.Add(numRequests)

	for i := 0; i < numRequests; i++ {
		go func() {
			defer wg.Done()

			req := httptest.NewRequest("GET", "/api/v1/ticket/validate/"+ticketID+"/"+userID, nil)
			rctx := chi.NewRouteContext()
			rctx.URLParams.Add("ticket_id", ticketID)
			rctx.URLParams.Add("user_id", userID)
			req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))

			rr := httptest.NewRecorder()
			s.validateHandler(rr, req)
			assert.Equal(t, http.StatusOK, rr.Code)
		}()
	}

	// Wait for all goroutines to finish
	wg.Wait()

	// Check that the ticket is in the tickets map
	_, ok := s.tickets.Load(ticketID)
	assert.True(t, ok)
}

// BenchmarkServer_ValidateHandler benchmarks the validateHandler with a single ticket.
func BenchmarkServer_ValidateHandler(b *testing.B) {
	s := &Server{
		expirationDuration: 10 * time.Second,
	}

	ticketID := "benchmark_ticket"
	userID := "benchmark_user"

	req := httptest.NewRequest("GET", "/api/v1/ticket/validate/"+ticketID+"/"+userID, nil)
	rctx := chi.NewRouteContext()
	rctx.URLParams.Add("ticket_id", ticketID)
	rctx.URLParams.Add("user_id", userID)
	req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		rr := httptest.NewRecorder()
		s.validateHandler(rr, req)
		if rr.Code != 200 {
			b.Fatalf("Expected status code 200, got %d", rr.Code)
		}
	}
}

// BenchmarkServer_ValidateHandler_Concurrent benchmarks validateHandler under concurrent load.
func BenchmarkServer_ValidateHandler_Concurrent(b *testing.B) {
	s := &Server{
		expirationDuration: 10 * time.Second,
	}

	ticketID := "benchmark_ticket_concurrent"
	userID := "benchmark_user"

	req := httptest.NewRequest("GET", "/api/v1/ticket/validate/"+ticketID+"/"+userID, nil)
	rctx := chi.NewRouteContext()
	rctx.URLParams.Add("ticket_id", ticketID)
	rctx.URLParams.Add("user_id", userID)
	req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))

	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			rr := httptest.NewRecorder()
			s.validateHandler(rr, req)
			if rr.Code != 200 {
				b.Errorf("Expected status code 200, got %d", rr.Code)
			}
		}
	})
}

// BenchmarkServer_MultipleTickets benchmarks validateHandler with multiple unique tickets.
func BenchmarkServer_MultipleTickets(b *testing.B) {
	s := &Server{
		expirationDuration: 10 * time.Second,
	}

	userID := "benchmark_user"

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		ticketID := "benchmark_ticket_" + strconv.Itoa(i)
		req := httptest.NewRequest("GET", "/api/v1/ticket/validate/"+ticketID+"/"+userID, nil)
		rctx := chi.NewRouteContext()
		rctx.URLParams.Add("ticket_id", ticketID)
		rctx.URLParams.Add("user_id", userID)
		req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))

		rr := httptest.NewRecorder()
		s.validateHandler(rr, req)
		if rr.Code != 200 {
			b.Fatalf("Expected status code 200, got %d", rr.Code)
		}
	}
}

// BenchmarkServer_ListHandler benchmarks the listHandler with a large number of tickets.
func BenchmarkServer_ListHandler(b *testing.B) {
	s := &Server{
		expirationDuration: 10 * time.Second,
	}

	// Pre-populate the server with a large number of tickets
	numTickets := 100000
	userID := "benchmark_user"

	for i := 0; i < numTickets; i++ {
		ticketID := "benchmark_ticket_" + strconv.Itoa(i)
		req := httptest.NewRequest("GET", "/api/v1/ticket/validate/"+ticketID+"/"+userID, nil)
		rctx := chi.NewRouteContext()
		rctx.URLParams.Add("ticket_id", ticketID)
		rctx.URLParams.Add("user_id", userID)
		req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))

		rr := httptest.NewRecorder()
		s.validateHandler(rr, req)
		if rr.Code != 200 {
			b.Fatalf("Expected status code 200, got %d", rr.Code)
		}
	}

	// Benchmark the listHandler
	reqList := httptest.NewRequest("GET", "/api/v1/ticket/list/anyspace", nil)
	rctxList := chi.NewRouteContext()
	rctxList.URLParams.Add("space", "anyspace")
	reqList = reqList.WithContext(context.WithValue(reqList.Context(), chi.RouteCtxKey, rctxList))

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		rrList := httptest.NewRecorder()
		s.listHandler(rrList, reqList)
		if rrList.Code != 200 {
			b.Fatalf("Expected status code 200, got %d", rrList.Code)
		}
	}
}
