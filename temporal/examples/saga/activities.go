package saga

import (
	"context"
	"fmt"

	"go.temporal.io/sdk/activity"
)

type TripReservationActivities struct{}

// ReserveCredit reserves credit from the account.
func (a *TripReservationActivities) ReserveCredit(ctx context.Context, amount float64) (string, error) {
	activity.GetLogger(ctx).Info("Reserving credit", "amount", amount)
	return fmt.Sprintf("Credit reserved: %.2f", amount), nil
}

// RefundCredit compensates credit reservation.
func (a *TripReservationActivities) RefundCredit(ctx context.Context, amount float64) error {
	activity.GetLogger(ctx).Info("Compensation: Refunding credit", "amount", amount)
	return nil
}

// BookHotel books a hotel room.
func (a *TripReservationActivities) BookHotel(ctx context.Context, hotelName string) (string, error) {
	activity.GetLogger(ctx).Info("Booking hotel", "hotelName", hotelName)
	return fmt.Sprintf("Hotel booked: %s", hotelName), nil
}

// CancelHotel cancels a hotel booking.
func (a *TripReservationActivities) CancelHotel(ctx context.Context, hotelName string) error {
	activity.GetLogger(ctx).Info("Compensation: Cancelling hotel booking", "hotelName", hotelName)
	return nil
}

// BookFlight books a flight.
func (a *TripReservationActivities) BookFlight(ctx context.Context, destination string) (string, error) {
	activity.GetLogger(ctx).Info("Booking flight", "destination", destination)
	if destination == "Fail" {
		return "", fmt.Errorf("flight to %s cancelled by airline (simulated failure)", destination)
	}
	return fmt.Sprintf("Flight booked to: %s", destination), nil
}

// CancelFlight cancels a flight booking.
func (a *TripReservationActivities) CancelFlight(ctx context.Context, destination string) error {
	activity.GetLogger(ctx).Info("Compensation: Cancelling flight booking", "destination", destination)
	return nil
}
