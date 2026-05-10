package scheduler

import (
	"context"
	"database/sql"
	"log"
	"time"

	pb_emp "github.com/RAF-SI-2025/EXBanka-4-Backend/shared/pb/employee"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

// ScheduleEOD schedules a job that runs every day at 23:59 to:
//  1. Snapshot each listing's current price into listing_daily_price_info.
//  2. Reset all actuary used_limit values to 0 via the employee-service.
func ScheduleEOD(db *sql.DB, employeeServiceAddr string) {
	scheduleNext(db, employeeServiceAddr)
	log.Println("eod: scheduled daily job at 23:59")
}

func scheduleNext(db *sql.DB, employeeServiceAddr string) {
	now := time.Now()
	next := time.Date(now.Year(), now.Month(), now.Day(), 23, 59, 0, 0, now.Location())
	if !now.Before(next) {
		next = next.Add(24 * time.Hour)
	}
	time.AfterFunc(time.Until(next), func() {
		runEOD(db, employeeServiceAddr)
		scheduleNext(db, employeeServiceAddr)
	})
}

func runEOD(db *sql.DB, employeeServiceAddr string) {
	log.Println("eod: running end-of-day job")

	// 1. Snapshot current prices.
	_, err := db.Exec(`
		INSERT INTO listing_daily_price_info (listing_id, date, price, ask, bid, change, volume)
		SELECT id, CURRENT_DATE, price, ask, bid, change, volume FROM listing
		ON CONFLICT (listing_id, date) DO NOTHING`)
	if err != nil {
		log.Printf("eod: snapshot prices: %v", err)
	} else {
		log.Println("eod: price snapshot complete")
	}

	// 2. Reset actuary used_limit via employee-service.
	if employeeServiceAddr == "" {
		log.Println("eod: EMPLOYEE_SERVICE_ADDR not set, skipping actuary reset")
		return
	}
	conn, err := grpc.NewClient(employeeServiceAddr, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		log.Printf("eod: dial employee-service: %v", err)
		return
	}
	defer func() {
		if err := conn.Close(); err != nil {
			log.Printf("eod: conn close: %v", err)
		}
	}()

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	client := pb_emp.NewEmployeeServiceClient(conn)
	if _, err := client.ResetAllActuaryUsedLimits(ctx, &pb_emp.ResetAllActuaryUsedLimitsRequest{}); err != nil {
		log.Printf("eod: reset actuary used limits: %v", err)
	} else {
		log.Println("eod: actuary used limits reset")
	}
}
