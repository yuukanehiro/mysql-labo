// main.go
package main

import (
	"fmt"
	"log"
	"runtime"
	"sync"
	"time"

	"math/rand"

	"github.com/oklog/ulid/v2"
	"gorm.io/driver/mysql"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

type DemoDeadlock struct {
	ID        string `gorm:"primaryKey"`
	CompanyID string `gorm:"index:idx_company_user,unique"`
	UserID    string `gorm:"index:idx_company_user,unique"`
	Data      string
}

func main() {
	runtime.GOMAXPROCS(10) // 並列実行性を最大化
	dsn := "root:root@tcp(127.0.0.1:3306)/test_db?parseTime=true&loc=Local"
	db, err := gorm.Open(mysql.Open(dsn), &gorm.Config{})
	if err != nil {
		log.Fatalf("failed to connect database: %v", err)
	}

	if err := db.Migrator().DropTable(&DemoDeadlock{}); err != nil {
		log.Fatalf("failed to drop table: %v", err)
	}

	if err := db.AutoMigrate(&DemoDeadlock{}); err != nil {
		log.Fatalf("migration failed: %v", err)
	}

	var wg sync.WaitGroup
	start := make(chan struct{})

	numCompanies := 10
	wg.Add(numCompanies)

	for i := 0; i < numCompanies; i++ {
		companyID := i + 1
		go func(cid int) {
			defer wg.Done()
			<-start
			log.Printf("Starting Tx for Company %d", cid)
			err := insertBatch(db, cid)
			if err != nil {
				log.Printf("Tx for Company %d failed: %v", cid, err)
			} else {
				log.Printf("Tx for Company %d completed", cid)
			}
		}(companyID)
	}

	time.Sleep(2 * time.Second)
	close(start)
	wg.Wait()
	fmt.Println("done")
}

func insertBatch(db *gorm.DB, companyID int) error {
	rentropy := ulid.Monotonic(rand.New(rand.NewSource(time.Now().UnixNano())), 0)
	companyULID := ulid.MustNew(ulid.Timestamp(time.Now()), rentropy).String()
	return db.Transaction(func(tx *gorm.DB) error {
		timestamp := time.Now().UnixNano()
		for batch := 0; batch < 20; batch++ {
			records := make([]*DemoDeadlock, 0, 1000)
			for i := 0; i < 1000; i++ {
				id := ulid.MustNew(ulid.Timestamp(time.Now()), rentropy).String()
				userULID := ulid.MustNew(ulid.Timestamp(time.Now()), rentropy).String()
				records = append(records, &DemoDeadlock{
					ID:        id,
					CompanyID: companyULID,
					UserID:    userULID,
					Data:      fmt.Sprintf("DATA-%d-%d", timestamp, rand.Intn(99999)),
				})
			}

			if err := tx.Clauses(clause.OnConflict{
				Columns:   []clause.Column{{Name: "company_id"}, {Name: "user_id"}},
				DoUpdates: clause.AssignmentColumns([]string{"data"}),
			}).CreateInBatches(records, 1000).Error; err != nil {
				return err
			}

			time.Sleep(50 * time.Millisecond) // 毎バッチで軽く遅延
		}
		return nil
	})
}
