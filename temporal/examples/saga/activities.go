package saga

import (
	"context"
	"fmt"

	"go.temporal.io/sdk/activity"
)

type TripReservationActivities struct{}

// ReserveCredit списывает средства со счета.
func (a *TripReservationActivities) ReserveCredit(ctx context.Context, amount float64) (string, error) {
	activity.GetLogger(ctx).Info("Списание средств", "amount", amount)
	return fmt.Sprintf("Credit reserved: %.2f", amount), nil
}

// RefundCredit компенсирует списание средств.
func (a *TripReservationActivities) RefundCredit(ctx context.Context, amount float64) error {
	activity.GetLogger(ctx).Info("Компенсация: Возврат средств", "amount", amount)
	return nil
}

// BookHotel бронирует номер в отеле.
func (a *TripReservationActivities) BookHotel(ctx context.Context, hotelName string) (string, error) {
	activity.GetLogger(ctx).Info("Бронирование отеля", "hotelName", hotelName)
	return fmt.Sprintf("Hotel booked: %s", hotelName), nil
}

// CancelHotel отменяет бронь отеля.
func (a *TripReservationActivities) CancelHotel(ctx context.Context, hotelName string) error {
	activity.GetLogger(ctx).Info("Компенсация: Отмена отеля", "hotelName", hotelName)
	return nil
}

// BookFlight покупает билет на самолет.
func (a *TripReservationActivities) BookFlight(ctx context.Context, destination string) (string, error) {
	activity.GetLogger(ctx).Info("Бронирование авиабилета", "destination", destination)
	if destination == "Fail" {
		return "", fmt.Errorf("рейс до %s отменен авиакомпанией (имитация сбоя)", destination)
	}
	return fmt.Sprintf("Flight booked to: %s", destination), nil
}

// CancelFlight отменяет авиабилет.
func (a *TripReservationActivities) CancelFlight(ctx context.Context, destination string) error {
	activity.GetLogger(ctx).Info("Компенсация: Отмена авиабилета", "destination", destination)
	return nil
}
