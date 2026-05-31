package temporal_test

import (
	"testing"
	"time"

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

// Test_HelloWorldWorkflow проверяет базовое выполнение приветственного процесса.
func (s *UnitTestSuite) Test_HelloWorldWorkflow() {
	env := s.NewTestWorkflowEnvironment()

	// Регистрируем активности, вызываемые внутри workflow
	env.RegisterActivity(helloworld.GreetActivity)

	// Запускаем Workflow в тестовом окружении
	env.ExecuteWorkflow(helloworld.GreetWorkflow, "World")

	s.True(env.IsWorkflowCompleted())
	s.NoError(env.GetWorkflowError())

	var result string
	err := env.GetWorkflowResult(&result)
	s.NoError(err)
	s.Equal("Hello, World!", result)
}

// Test_SubscriptionWorkflow проверяет обработку сигналов и запросов в SubscriptionWorkflow.
func (s *UnitTestSuite) Test_SubscriptionWorkflow() {
	env := s.NewTestWorkflowEnvironment()

	// Запускаем асинхронное выполнение, чтобы мы могли посылать сигналы в процессе
	env.RegisterWorkflow(signal.SubscriptionWorkflow)

	// Регистрируем отложенную отправку сигналов
	env.RegisterDelayedCallback(func() {
		// Проверяем начальный статус через Query
		val, err := env.QueryWorkflow("GetSubscriptionStatus")
		s.NoError(err)
		var status signal.SubscriptionStatus
		err = val.Get(&status)
		s.NoError(err)
		s.Equal("Active", status.State)

		// Отправляем сигнал обновления биллинга
		env.SignalWorkflow("UpdateBillingInfo", "Visa Card")
	}, 10*time.Second)

	env.RegisterDelayedCallback(func() {
		// Проверяем промежуточный статус через Query
		val, err := env.QueryWorkflow("GetSubscriptionStatus")
		s.NoError(err)
		var status signal.SubscriptionStatus
		err = val.Get(&status)
		s.NoError(err)
		s.Equal("Updated", status.State)
		s.Equal("Visa Card", status.BillingInfo)

		// Отправляем сигнал отмены подписки
		env.SignalWorkflow("CancelSubscription", nil)
	}, 20*time.Second)

	// Запускаем Workflow на 1 минуту
	env.ExecuteWorkflow(signal.SubscriptionWorkflow, 1*time.Minute)

	s.True(env.IsWorkflowCompleted())
	s.NoError(env.GetWorkflowError())

	var finalResult string
	err := env.GetWorkflowResult(&finalResult)
	s.NoError(err)
	s.Equal("Cancelled", finalResult)
}

// Test_SagaReservation_Success проверяет успешное бронирование без отката.
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

// Test_SagaReservation_Fail проверяет работу Saga Pattern и откат транзакций при сбое.
func (s *UnitTestSuite) Test_SagaReservation_Fail() {
	env := s.NewTestWorkflowEnvironment()
	var a *saga.TripReservationActivities

	// Регистрируем транзакции и их компенсации
	env.RegisterActivity(a.ReserveCredit)
	env.RegisterActivity(a.RefundCredit)
	env.RegisterActivity(a.BookHotel)
	env.RegisterActivity(a.CancelHotel)
	env.RegisterActivity(a.BookFlight)
	env.RegisterActivity(a.CancelFlight)

	params := saga.TripReservationParams{
		Amount:      250.0,
		HotelName:   "Hostel 123",
		Destination: "Fail", // Вызовет ошибку в BookFlight
	}

	// Отслеживаем вызовы компенсаций
	var refundCalled, cancelHotelCalled bool
	env.OnActivity(a.RefundCredit, mock.Anything, 250.0).Return(nil).Run(func(args mock.Arguments) {
		refundCalled = true
	})
	env.OnActivity(a.CancelHotel, mock.Anything, "Hostel 123").Return(nil).Run(func(args mock.Arguments) {
		cancelHotelCalled = true
	})

	env.ExecuteWorkflow(saga.TripReservationWorkflow, params)

	s.True(env.IsWorkflowCompleted())
	// Воркфлоу должен завершиться с ошибкой, так как последний шаг упал
	s.Error(env.GetWorkflowError())

	// Проверяем, что компенсации были вызваны в процессе отката саги
	s.True(refundCalled, "Должен быть вызван возврат средств (RefundCredit)")
	s.True(cancelHotelCalled, "Должна быть вызвана отмена отеля (CancelHotel)")
}
