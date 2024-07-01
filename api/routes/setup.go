package route

import (
	"build-machine/api/controller"
	"build-machine/internal"
	"fmt"
	"log"
	"net/http"

	"github.com/rs/cors"
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

	mux.Handle("/healthcheck", http.HandlerFunc(c.HealthCheck))

	mux.Handle("/deploy", http.HandlerFunc(c.DeployFromS3Workflow))

	mux.Handle("/github-deploy", http.HandlerFunc(c.DeployFromGithubWorkflow))

	mux.Handle("/deploy-empty-project", http.HandlerFunc(c.DeployEmptyProjectWorkflow))

	serverPort := internal.GetConfig().ServerPort
	fmt.Println("Server running on port", serverPort)

	cors.AllowAll().Handler(mux)
	log.Fatal(http.ListenAndServe(fmt.Sprintf(":%s", serverPort), mux))
}
