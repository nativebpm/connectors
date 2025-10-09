package store

import "sync"

// Application holds the loan application data that we keep out of Camunda
type Application struct {
	ApplicationNumber int
	ApplicantName     string
	ApplicantEmail    string
	RequestedAmount   float64
	LoanPurpose       string
	LoanTerm          int
	MonthlyIncome     float64
	ExistingDebts     float64
	EmploymentYears   int
	SubmittedAtUnix   int64
	// Results collected from loanGranter / requestRejecter tasks
	Results []Result
}

// Store is a simple in-memory store for application data.
// It's intentionally minimal for example/demo use only.
type Store struct {
	mu   sync.RWMutex
	data map[string]Application
}

// New creates a new in-memory Store
func New() *Store {
	return &Store{data: make(map[string]Application)}
}

// Save stores an application under the given businessKey
func (s *Store) Save(businessKey string, app Application) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.data[businessKey] = app
}

// Get returns an application and true if found
func (s *Store) Get(businessKey string) (Application, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	a, ok := s.data[businessKey]
	return a, ok
}

// Result represents the outcome of a single credit evaluation (one score)
type Result struct {
	Score          int
	LoanGranted    bool
	ApprovedAmount float64
	InterestRate   float64
	Message        string
}

// AppendResult appends a result to the application identified by businessKey.
// If the application does not exist, this is a no-op.
func (s *Store) AppendResult(businessKey string, r Result) {
	s.mu.Lock()
	defer s.mu.Unlock()
	app, ok := s.data[businessKey]
	if !ok {
		return
	}
	app.Results = append(app.Results, r)
	s.data[businessKey] = app
}

// GetResults returns the results slice for the given businessKey.
func (s *Store) GetResults(businessKey string) ([]Result, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	app, ok := s.data[businessKey]
	if !ok {
		return nil, false
	}
	return app.Results, true
}
