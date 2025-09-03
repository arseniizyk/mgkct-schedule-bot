package main

import (
	"encoding/json"
	"log"
	"net"
	"net/http"

	pb "github.com/arseniizyk/mgkct-schedule-bot/libs/proto"
	"github.com/arseniizyk/mgkct-schedule-bot/services/scraper/internal/config"
	"github.com/arseniizyk/mgkct-schedule-bot/services/scraper/pkg/crawler"
	"github.com/arseniizyk/mgkct-schedule-bot/services/scraper/pkg/server"
	"google.golang.org/grpc"
)

func main() {
	cfg, err := config.New()
	if err != nil {
		log.Fatal(err)
	}

	c := crawler.New()
	schedule, err := c.Crawl()
	if err != nil {
		log.Println("crawl error:", err)
		return
	}

	lis, err := net.Listen("tcp", ":"+cfg.GRPCPort)
	if err != nil {
		log.Fatal(err)
	}

	grpcServer := grpc.NewServer()
	pb.RegisterScheduleServiceServer(grpcServer, server.New(schedule))

	go func() {
		log.Println("gRPC server started on port ", cfg.GRPCPort)
		if err := grpcServer.Serve(lis); err != nil {
			log.Fatal(err)
		}
	}()

	http.HandleFunc("/test", func(w http.ResponseWriter, r *http.Request) {
		b, err := json.MarshalIndent(schedule, "", " ")
		if err != nil {
			http.Error(w, "failed to marshal schedule", http.StatusInternalServerError)
			log.Println("marshal error:", err)
			return
		}

		w.Header().Set("Content-Type", "application/json")
		w.Write(b)
	})

	log.Println("HTTP server started on port", cfg.HttpPort)
	log.Fatal(http.ListenAndServe(":"+cfg.HttpPort, nil))
}
