package choochoo

import (
	"context"
	"log"
	"os"

	"cloud.google.com/go/firestore"
	firebase "firebase.google.com/go"
)

var (
	fbApp  *firebase.App
	fstore *firestore.Client
)

func init() {
	var err error
	ctx := context.Background()

	projectID := os.Getenv("GCP_PROJECT")
	fbApp, err = firebase.NewApp(ctx, &firebase.Config{ProjectID: projectID})
	if err != nil {
		log.Fatalf("Cannot initialize Firebase: %s", err)
	}

	fstore, err = fbApp.Firestore(ctx)
	if err != nil {
		log.Fatalf("Cannot initialize Firestore: %s", err)
	}
}
