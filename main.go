package happeninghound

import (
	"context"
	"github.com/johtani/happeninghound/client"
	"log"
)

func main() {
	if err := client.Run(context.Background()); err != nil {
		log.Fatal(err)
	}
}
