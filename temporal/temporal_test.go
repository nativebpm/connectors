package temporal_test

import (
	"testing"
	"time"

	"github.com/nativebpm/connectors/temporal"
	"github.com/nativebpm/connectors/temporal/examples/helloworld"
	"github.com/nativebpm/connectors/temporal/examples/saga"
	"github.com/nativebpm/connectors/temporal/examples/signal"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/suite"
	"go.temporal.io/sdk/testsuite"
)

type UnitTestSuite struct {
	suite.Suite
	testsuite.WorkflowTestSuite
}

func TestUnitTestSuite(t *testing.T) {
	suite.Run(t, new(UnitTestSuite))
}

// Test_HelloWorldWorkflow verifies the basic execution of the greeting process.
func (s *UnitTestSuite) Test_HelloWorldWorkflow() {
	env := s.NewTestWorkflowEnvironment()

	// Register activities invoked inside the workflow
	env.RegisterActivity(helloworld.GreetActivity)

	// Run Workflow in the test environment
	env.ExecuteWorkflow(helloworld.GreetWorkflow, "World")

	s.True(env.IsWorkflowCompleted())
	s.NoError(env.GetWorkflowError())

	var result string
	err := env.GetWorkflowResult(&result)
	s.NoError(err)
	s.Equal("Hello, World!", result)
}

// Test_SubscriptionWorkflow verifies handling of signals and queries in SubscriptionWorkflow.
func (s *UnitTestSuite) Test_SubscriptionWorkflow() {
	env := s.NewTestWorkflowEnvironment()

	// Run asynchronously so we can send signals during execution
	env.RegisterWorkflow(signal.SubscriptionWorkflow)

	// Schedule delayed signals
	env.RegisterDelayedCallback(func() {
		// Verify initial status via Query
		val, err := env.QueryWorkflow("GetSubscriptionStatus")
		s.NoError(err)
		var status signal.SubscriptionStatus
		err = val.Get(&status)
		s.NoError(err)
		s.Equal("Active", status.State)

		// Send billing update signal
		env.SignalWorkflow("UpdateBillingInfo", "Visa Card")
	}, 10*time.Second)

	env.RegisterDelayedCallback(func() {
		// Verify intermediate status via Query
		val, err := env.QueryWorkflow("GetSubscriptionStatus")
		s.NoError(err)
		var status signal.SubscriptionStatus
		err = val.Get(&status)
		s.NoError(err)
		s.Equal("Updated", status.State)
		s.Equal("Visa Card", status.BillingInfo)

		// Send subscription cancellation signal
		env.SignalWorkflow("CancelSubscription", nil)
	}, 20*time.Second)

	// Start Workflow for 1 minute
	env.ExecuteWorkflow(signal.SubscriptionWorkflow, 1*time.Minute)

	s.True(env.IsWorkflowCompleted())
	s.NoError(env.GetWorkflowError())

	var finalResult string
	err := env.GetWorkflowResult(&finalResult)
	s.NoError(err)
	s.Equal("Cancelled", finalResult)
}

// Test_SagaReservation_Success verifies successful booking without compensation.
func (s *UnitTestSuite) Test_SagaReservation_Success() {
	env := s.NewTestWorkflowEnvironment()
	var a *saga.TripReservationActivities

	env.RegisterActivity(a.ReserveCredit)
	env.RegisterActivity(a.BookHotel)
	env.RegisterActivity(a.BookFlight)

	params := saga.TripReservationParams{
		Amount:      500.0,
		HotelName:   "Luxury Resort",
		Destination: "Paris",
	}

	env.ExecuteWorkflow(saga.TripReservationWorkflow, params)

	s.True(env.IsWorkflowCompleted())
	s.NoError(env.GetWorkflowError())

	var result string
	err := env.GetWorkflowResult(&result)
	s.NoError(err)
	s.Equal("Trip successfully reserved!", result)
}

// Test_SagaReservation_Fail verifies Saga Pattern and transaction rollbacks on failure.
func (s *UnitTestSuite) Test_SagaReservation_Fail() {
	env := s.NewTestWorkflowEnvironment()
	var a *saga.TripReservationActivities

	// Register transactions and their compensations
	env.RegisterActivity(a.ReserveCredit)
	env.RegisterActivity(a.RefundCredit)
	env.RegisterActivity(a.BookHotel)
	env.RegisterActivity(a.CancelHotel)
	env.RegisterActivity(a.BookFlight)
	env.RegisterActivity(a.CancelFlight)

	params := saga.TripReservationParams{
		Amount:      250.0,
		HotelName:   "Hostel 123",
		Destination: "Fail", // Will trigger an error in BookFlight
	}

	// Track compensation calls
	var refundCalled, cancelHotelCalled bool
	env.OnActivity(a.RefundCredit, mock.Anything, 250.0).Return(nil).Run(func(args mock.Arguments) {
		refundCalled = true
	})
	env.OnActivity(a.CancelHotel, mock.Anything, "Hostel 123").Return(nil).Run(func(args mock.Arguments) {
		cancelHotelCalled = true
	})

	env.ExecuteWorkflow(saga.TripReservationWorkflow, params)

	s.True(env.IsWorkflowCompleted())
	// Workflow should fail since the last step failed
	s.Error(env.GetWorkflowError())

	// Verify compensations were called during saga rollback
	s.True(refundCalled, "RefundCredit should be called")
	s.True(cancelHotelCalled, "CancelHotel should be called")
}

// Test_CDCWorkflow verifies correct execution of a workflow via CDC delegation.
func (s *UnitTestSuite) Test_CDCWorkflow() {
	env := s.NewTestWorkflowEnvironment()
	var a *temporal.CDCActivities

	// Register Workflow and Activity
	env.RegisterWorkflow(temporal.GreetCDCWorkflow)

	// Mock delegation activity (returns nil, i.e. DB write succeeded)
	env.OnActivity(a.DelegateToSequin, mock.Anything, "greet-cdc", "Temporal CDC User").Return(nil)

	// Schedule sending signal after 5 seconds of virtual time
	env.RegisterDelayedCallback(func() {
		// Simulate CDC worker sending result signal
		env.SignalWorkflow("TaskCompletedSignal", "Hello, Temporal CDC User (via CDC)!")
	}, 5*time.Second)

	env.ExecuteWorkflow(temporal.GreetCDCWorkflow, "Temporal CDC User")

	s.True(env.IsWorkflowCompleted())
	s.NoError(env.GetWorkflowError())

	var result string
	err := env.GetWorkflowResult(&result)
	s.NoError(err)
	s.Equal("Hello, Temporal CDC User (via CDC)!", result)
}
