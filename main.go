package main

import (
	"container/heap"
	"fmt"
	"net/http"
	"strconv"
	"sync"
	"time"
)

// Token represents an order with priority
type Token struct {
	ID        int
	Item      string
	Priority  int       // Lower values indicate higher priority
	Status    string    // "preparing" or "prepared"
	Timestamp time.Time // Time of order, used to resolve ties in priority
	index     int       // Index in the heap
}

// PriorityQueue implements a priority queue for Tokens
type PriorityQueue []*Token

// Len, Less, and Swap methods to satisfy the heap.Interface
func (pq PriorityQueue) Len() int { return len(pq) }
func (pq PriorityQueue) Less(i, j int) bool {
	// Order by priority, then by timestamp
	if pq[i].Priority == pq[j].Priority {
		return pq[i].Timestamp.Before(pq[j].Timestamp)
	}
	return pq[i].Priority < pq[j].Priority
}
func (pq PriorityQueue) Swap(i, j int) {
	pq[i], pq[j] = pq[j], pq[i]
	pq[i].index, pq[j].index = i, j
}

// Push and Pop methods for heap
func (pq *PriorityQueue) Push(x interface{}) {
	n := len(*pq)
	token := x.(*Token)
	token.index = n
	*pq = append(*pq, token)
}

func (pq *PriorityQueue) Pop() interface{} {
	old := *pq
	n := len(old)
	token := old[n-1]
	old[n-1] = nil // Avoid memory leak
	token.index = -1
	*pq = old[0 : n-1]
	return token
}

// OrderManager manages tokens and priorities
type OrderManager struct {
	tokens     PriorityQueue
	prepared   []*Token
	counter    int
	mu         sync.Mutex
	preparedMu sync.Mutex
}

func NewOrderManager() *OrderManager {
	pq := make(PriorityQueue, 0)
	heap.Init(&pq)
	return &OrderManager{
		tokens: pq,
	}
}

// AddOrder creates a new order and places it in the priority queue
func (om *OrderManager) AddOrder(item string, priority int) *Token {
	om.mu.Lock()
	defer om.mu.Unlock()
	om.counter++
	token := &Token{
		ID:        om.counter,
		Item:      item,
		Priority:  priority,
		Status:    "preparing",
		Timestamp: time.Now(),
	}
	heap.Push(&om.tokens, token)
	return token
}

// PrepareOrder marks the top order as prepared
func (om *OrderManager) PrepareOrder() *Token {
	om.mu.Lock()
	defer om.mu.Unlock()
	if om.tokens.Len() == 0 {
		return nil
	}
	token := heap.Pop(&om.tokens).(*Token)
	token.Status = "prepared"

	om.preparedMu.Lock()
	om.prepared = append(om.prepared, token)
	om.preparedMu.Unlock()

	return token
}

// ListOrders lists preparing and prepared orders
func (om *OrderManager) ListOrders() ([]*Token, []*Token) {
	om.mu.Lock()
	defer om.mu.Unlock()
	om.preparedMu.Lock()
	defer om.preparedMu.Unlock()

	preparing := make([]*Token, om.tokens.Len())
	copy(preparing, om.tokens)

	prepared := make([]*Token, len(om.prepared))
	copy(prepared, om.prepared)

	return preparing, prepared
}

// HTTP handlers
func (om *OrderManager) addOrderHandler(w http.ResponseWriter, r *http.Request) {
	item := r.URL.Query().Get("item")
	priorityStr := r.URL.Query().Get("priority")
	priority, err := strconv.Atoi(priorityStr)
	if err != nil {
		http.Error(w, "Invalid priority", http.StatusBadRequest)
		return
	}
	token := om.AddOrder(item, priority)
	fmt.Fprintf(w, "Order received: ID=%d, Item=%s, Priority=%d\n", token.ID, token.Item, token.Priority)
}

func (om *OrderManager) prepareOrderHandler(w http.ResponseWriter, r *http.Request) {
	token := om.PrepareOrder()
	if token == nil {
		fmt.Fprintln(w, "No orders to prepare")
		return
	}
	fmt.Fprintf(w, "Order prepared: ID=%d, Item=%s\n", token.ID, token.Item)
}

func (om *OrderManager) listOrdersHandler(w http.ResponseWriter, r *http.Request) {
	preparing, prepared := om.ListOrders()

	fmt.Fprintln(w, "Preparing Orders:")
	for _, token := range preparing {
		fmt.Fprintf(w, "ID=%d, Item=%s, Priority=%d\n", token.ID, token.Item, token.Priority)
	}

	fmt.Fprintln(w, "\nPrepared Orders:")
	for _, token := range prepared {
		fmt.Fprintf(w, "ID=%d, Item=%s\n", token.ID, token.Item)
	}
}

func main() {
	om := NewOrderManager()
	http.HandleFunc("/addOrder", om.addOrderHandler)
	http.HandleFunc("/prepareOrder", om.prepareOrderHandler)
	http.HandleFunc("/listOrder", om.listOrdersHandler)

	fmt.Println("Server starting at http://localhost:8080")
	http.ListenAndServe(":8080", nil)
}
