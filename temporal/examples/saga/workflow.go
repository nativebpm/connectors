package saga

import (
	"time"

	"go.temporal.io/sdk/workflow"
)

type TripReservationParams struct {
	Amount      float64
	HotelName   string
	Destination string
}

// TripReservationWorkflow координирует транзакции и компенсации по паттерну Saga.
func TripReservationWorkflow(ctx workflow.Context, params TripReservationParams) (string, error) {
	options := workflow.ActivityOptions{
		StartToCloseTimeout: 5 * time.Second,
	}
	ctx = workflow.WithActivityOptions(ctx, options)

	var activities *TripReservationActivities
	logger := workflow.GetLogger(ctx)
	
	// Список компенсаций, которые нужно выполнить в случае сбоя (LIFO)
	var compensations []func(ctx workflow.Context) error

	defer func() {
		// Если функция завершилась с ошибкой (например, паника или ошибка выполнения),
		// выполняем накопленный стек компенсаций.
		// В Temporal важно использовать workflow.NewDisconnectedContext, чтобы выполнять
		// компенсации даже если родительский контекст workflow был отменен (например, по таймауту).
	}()

	// 1. Списание средств
	var creditResult string
	err := workflow.ExecuteActivity(ctx, activities.ReserveCredit, params.Amount).Get(ctx, &creditResult)
	if err != nil {
		logger.Error("Ошибка при ReserveCredit", "error", err)
		return "", err
	}
	// Добавляем компенсацию в начало стека
	compensations = append(compensations, func(ctx workflow.Context) error {
		return workflow.ExecuteActivity(ctx, activities.RefundCredit, params.Amount).Get(ctx, nil)
	})

	// 2. Бронирование отеля
	var hotelResult string
	err = workflow.ExecuteActivity(ctx, activities.BookHotel, params.HotelName).Get(ctx, &hotelResult)
	if err != nil {
		logger.Error("Ошибка при BookHotel, инициируем откат саги", "error", err)
		executeCompensations(ctx, compensations)
		return "", err
	}
	// Добавляем компенсацию
	compensations = append(compensations, func(ctx workflow.Context) error {
		return workflow.ExecuteActivity(ctx, activities.CancelHotel, params.HotelName).Get(ctx, nil)
	})

	// 3. Бронирование перелета
	var flightResult string
	err = workflow.ExecuteActivity(ctx, activities.BookFlight, params.Destination).Get(ctx, &flightResult)
	if err != nil {
		logger.Error("Ошибка при BookFlight, инициируем откат саги", "error", err)
		executeCompensations(ctx, compensations)
		return "", err
	}

	logger.Info("Бронирование тура успешно завершено!")
	return "Trip successfully reserved!", nil
}

// executeCompensations запускает компенсации в обратном порядке (LIFO)
func executeCompensations(ctx workflow.Context, compensations []func(workflow.Context) error) {
	// Создаем изолированный контекст для выполнения компенсаций, 
	// чтобы они не прервались, если основной контекст был отменен.
	disconnectedCtx, _ := workflow.NewDisconnectedContext(ctx)
	logger := workflow.GetLogger(ctx)

	logger.Info("Запуск компенсирующих транзакций (откат саги)...")
	for i := len(compensations) - 1; i >= 0; i-- {
		err := compensations[i](disconnectedCtx)
		if err != nil {
			logger.Error("Ошибка при выполнении компенсирующей транзакции", "index", i, "error", err)
			// В реальных системах здесь может потребоваться эскалация или очередь ручного разбора
		}
	}
}
