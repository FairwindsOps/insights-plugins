package main

import (
	"context"
	"fmt"

	"github.com/fairwindsops/insights-plugins/prometheus/pkg/data"
)

func main() {
	fmt.Println("vim-go")
	address := "http://localhost:8080"
	client, err := data.GetClient(address)
	if err != nil {
		panic(err)
	}
	res, err := data.GetMetrics(context.Background(), client)
	if err != nil {
		panic(err)
	}
	fmt.Printf("%+v", data.CalculateStatistics(res))
}
