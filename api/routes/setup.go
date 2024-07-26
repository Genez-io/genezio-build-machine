package route

import (
	"build-machine/api/controller"
	"build-machine/internal"
	"fmt"
	"log"
	"net/http"
)

func CORS(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		w.Header().Add("Access-Control-Allow-Origin", "*")
		w.Header().Add("Access-Control-Allow-Credentials", "true")
		w.Header().Add("Access-Control-Allow-Headers", "Content-Type, Content-Length, Accept-Encoding, X-CSRF-Token, Authorization, accept, origin, Cache-Control, X-Requested-With")
		w.Header().Add("Access-Control-Allow-Methods", "POST, GET, OPTIONS, PUT, DELETE")

		if r.Method == "OPTIONS" {
			http.Error(w, "No Content", http.StatusNoContent)
			return
		}

		next(w, r)
	}
}

func SetupHTTP() {
	c := controller.NewDeploymentsController()
	mux := http.NewServeMux()

	mux.Handle("/healthcheck", http.HandlerFunc(CORS(c.HealthCheck)))
	mux.Handle("/deploy", http.HandlerFunc(CORS(c.Deploy)))
	mux.Handle("/state/{job_id}", http.HandlerFunc(CORS(c.GetState)))
	serverPort := internal.GetConfig().ServerPort
	fmt.Println("Server running on port", serverPort)

	log.Fatal(http.ListenAndServe(fmt.Sprintf(":%s", serverPort), mux))
}
