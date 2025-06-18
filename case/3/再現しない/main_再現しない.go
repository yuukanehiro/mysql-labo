// main.go
package main

import (
	"fmt"
	"log"
	"math/rand"
	"sync"
	"time"

	"gorm.io/driver/mysql"
	"gorm.io/gorm"
)

type DemoDeadlock struct {
	ID        string `gorm:"primaryKey"`
	CompanyID int
	UserID    int
	Data      string
}

func main() {
	dsn := "root:root@tcp(127.0.0.1:3306)/test_db?parseTime=true&loc=Local"
	db, err := gorm.Open(mysql.Open(dsn), &gorm.Config{})
	if err != nil {
		log.Fatalf("failed to connect database: %v", err)
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
		prefix := fmt.Sprintf("C%02d", companyID)
		go func(cid int, pfx string) {
			defer wg.Done()
			<-start
			log.Printf("Starting Tx for Company %d", cid)
			err := insertBatch(db, cid, pfx)
			if err != nil {
				log.Printf("Tx for Company %d failed: %v", cid, err)
			} else {
				log.Printf("Tx for Company %d completed", cid)
			}
		}(companyID, prefix)
	}

	time.Sleep(2 * time.Second)
	close(start)
	wg.Wait()
	fmt.Println("done")
}

func insertBatch(db *gorm.DB, companyID int, prefix string) error {
	return db.Transaction(func(tx *gorm.DB) error {
		timestamp := time.Now().UnixNano()
		for batch := 0; batch < 20; batch++ {
			records := make([]*DemoDeadlock, 0, 1000)
			for i := 0; i < 1000; i++ {
				id := fmt.Sprintf("%s%08d", prefix, batch*1000+i)
				records = append(records, &DemoDeadlock{
					ID:        id,
					CompanyID: companyID,
					UserID:    100000*companyID + batch*1000 + i,
					Data:      fmt.Sprintf("DATA-%d-%d", timestamp, rand.Intn(99999)),
				})
			}
			if err := tx.CreateInBatches(records, 1000).Error; err != nil {
				return err
			}
		}
		return nil
	})
}
