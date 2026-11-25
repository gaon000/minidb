package main

import (
	"errors"
	"fmt"
	"log"
	"os"

	"github.com/gaon000/minidb"
)

func main() {
	if err := run(os.Args[1:]); err != nil {
		log.Fatal(err)
	}
}

func run(args []string) (err error) {
	if len(args) != 2 || (args[0] != "write" && args[0] != "read") {
		return errors.New("usage: lesson1 <write|read> <db-file>")
	}
	storage, err := minidb.Open(args[1])
	if err != nil {
		return err
	}
	defer func() { err = errors.Join(err, storage.Close()) }()

	if args[0] == "read" {
		const rid = 2
		user, err := storage.Get(rid)
		if err != nil {
			return err
		}
		fmt.Printf("RID %d: %s (ID %d, age %d)\n", rid, user.Name, user.ID, user.Age)
		return nil
	}

	users := []minidb.User{
		{ID: 101, Name: "Alice", Age: 25},
		{ID: 42, Name: "Bob", Age: 30},
		{ID: 98, Name: "Charlie", Age: 28},
	}
	for _, user := range users {
		rid, err := storage.Insert(user)
		if err != nil {
			return err
		}
		fmt.Printf("Inserted %s at RID %d\n", user.Name, rid)
	}
	return nil
}
