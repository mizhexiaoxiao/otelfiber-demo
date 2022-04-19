package main

import (
	"fmt"
	"log"
	"math/rand"
	"net/http"
	"os"
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/mizhexiaoxiao/otelfiber-demo/pkg"
	"go.opentelemetry.io/contrib/propagators/b3"
	"go.opentelemetry.io/otel"
)

const (
	//collectorEndpoint = "http://tracing-analysis-dc-hz.aliyuncs.com/adapt_cs0r1qyg7e@00f73f3e6fa421d_cs0r1qyg7e@53df7ad2afe8301/api/traces"
	collectorEndpoint = "http://127.0.0.1:3001"
)

var (
	projectName = GetEnvDefault("PROJECT_NAME", "fiber")
	namespace   = GetEnvDefault("NAMESPACE", "default")
	serviceName = projectName + "." + namespace
	tracer      = otel.Tracer(serviceName)
)

func main() {
	shutdown := pkg.InitializeGlobalTracer(serviceName, collectorEndpoint)
	defer shutdown()

	b3prop := b3.New(b3.WithInjectEncoding(b3.B3MultipleHeader))

	app := fiber.New()
	// app.Use(otelfiber.Middleware(serviceName, otelfiber.WithPropagators(b3prop)))
	app.Use(pkg.Middleware(serviceName, pkg.WithPropagators(b3prop)))

	app.Get("/users/:id", GetUserId)
	app.Get("/service", serviceHander)

	log.Fatal(app.Listen(":3000"))
}

func GetEnvDefault(key, defVal string) string {
	val, exist := os.LookupEnv(key)
	if !exist {
		return defVal
	}
	return val
}

func GetUserId(c *fiber.Ctx) error {
	id := c.Params("id")

	parentCtx := c.UserContext()
	_, span := tracer.Start(parentCtx, fmt.Sprintf("Get UserId %s", id))
	defer span.End()

	//do req
	go func() {
		err := asyncReqHandler(c)
		if err != nil {
			fmt.Println("request faild", err)
		}
	}()

	//db span

	return c.JSON(fiber.Map{"id:": id})
}

func asyncReqHandler(c *fiber.Ctx) error {
	req, _ := http.NewRequest("GET", "http://localhost:3000/service", nil)
	// get trace header from context
	carrier := pkg.GetOtelSpanHeaders(c)
	// inject span context in request header
	req.Header = carrier
	fmt.Printf("Sending request...\n")
	client := http.Client{}
	// do request
	_, err := client.Do(req)
	if err != nil {
		return err
	}
	time.Sleep(time.Duration(rand.Intn(200) * int(time.Millisecond)))
	return nil
}

// func asyncReqHandler(c *fiber.Ctx) error {
// 	// Build request
// 	asyncReq, _ := http.NewRequest("GET", "https://www.baidu.com", nil)
// 	// Inject span context in reqeust header
// 	carrier := pkg.GetOtelSpanHeaders(c)
// 	asyncReq.Header = carrier
// 	// async req
// 	if _, err := http.DefaultClient.Do(asyncReq); err != nil {
// 		// request err, set err tag
// 		return err
// 	}
// 	time.Sleep(time.Duration(rand.Intn(200) * int(time.Millisecond)))
// 	return nil
// }

func serviceHander(c *fiber.Ctx) error {
	fmt.Println("received request header", c.GetReqHeaders())
	fmt.Println("request service successfully")
	return nil
}
