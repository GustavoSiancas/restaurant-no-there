package main

import (
	"context"
	"fmt"
	"log"

	"backend/internal/config"
	"github.com/jackc/pgx/v5/pgxpool"
)

func main() {
	cfg, err := config.Load()
	if err != nil {
		log.Fatal(err)
	}
	ctx := context.Background()
	db, err := pgxpool.New(ctx, cfg.DatabaseURL)
	if err != nil {
		log.Fatal(err)
	}
	defer db.Close()
	tx, err := db.Begin(ctx)
	if err != nil {
		log.Fatal(err)
	}
	defer func() { _ = tx.Rollback(ctx) }()
	created, skipped := 0, 0
	for number := 1; number <= 50; number++ {
		dni := fmt.Sprintf("900%05d", number)
		employeeCode := fmt.Sprintf("SEED-%03d", number)
		var exists bool
		err = tx.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM user_credentials WHERE type='DNI' AND identifier=$1) OR EXISTS(SELECT 1 FROM worker_information WHERE employee_code=$2)`, dni, employeeCode).Scan(&exists)
		if err != nil {
			log.Fatal(err)
		}
		if exists {
			skipped++
			continue
		}
		var userID string
		err = tx.QueryRow(ctx, `INSERT INTO users(role) VALUES('WORKER') RETURNING id`).Scan(&userID)
		if err != nil {
			log.Fatal(err)
		}
		firstName := fmt.Sprintf("Trabajador %02d", number)
		email := fmt.Sprintf("worker%02d@example.test", number)
		if _, err = tx.Exec(ctx, `INSERT INTO user_profiles(user_id,first_name,last_name,email) VALUES($1,$2,'Prueba',$3)`, userID, firstName, email); err != nil {
			log.Fatal(err)
		}
		if _, err = tx.Exec(ctx, `INSERT INTO user_credentials(user_id,type,identifier) VALUES($1,'DNI',$2)`, userID, dni); err != nil {
			log.Fatal(err)
		}
		if _, err = tx.Exec(ctx, `INSERT INTO worker_information(user_id,employee_code,job_title,department,hire_date,notes) VALUES($1,$2,'Colaborador','Operaciones',CURRENT_DATE,'Generado por seed de desarrollo')`, userID, employeeCode); err != nil {
			log.Fatal(err)
		}
		created++
	}
	if err = tx.Commit(ctx); err != nil {
		log.Fatal(err)
	}
	log.Printf("workers seed completed: created=%d skipped=%d", created, skipped)
}
