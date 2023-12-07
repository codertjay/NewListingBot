package controllers

import (
	"NewListingBot/adapters"
	"NewListingBot/database"
	"NewListingBot/models"
	"NewListingBot/serializers"
	"context"
	"github.com/gofiber/fiber/v2"
	"gorm.io/gorm/clause"
	"time"
)

type Response struct {
	Message any  `json:"message,omitempty"`
	Success bool `json:"success,omitempty"`
	Error   any  `json:"error,omitempty"`
	Errors  any  `json:"errors,omitempty"`
	Detail  any  `json:"detail,omitempty"` // used by the backend developer
}

func OrderListController(c *fiber.Ctx) error {
	var orders []models.Order

	var ctx, cancel = context.WithTimeout(context.Background(), 100*time.Second)
	defer cancel()

	// Open the database connection
	db := database.DBConnection()
	defer database.CloseDB()

	err := db.WithContext(ctx).Model(&models.Order{}).
		Order(clause.OrderByColumn{Column: clause.Column{Name: "timestamp"}, Desc: true}).Find(&orders).Error
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).SendString("Error fetching orders")
	}

	return c.Status(200).JSON(orders)
}

func OrderCreateController(c *fiber.Ctx) error {
	var order models.Order
	var requestBody serializers.OrderCreateRequestSerializer

	var ctx, cancel = context.WithTimeout(context.Background(), 100*time.Second)
	defer cancel()

	// Open the database connection
	db := database.DBConnection()
	defer database.CloseDB()

	validateAdapter := adapters.NewValidate()

	if err := c.BodyParser(&requestBody); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(Response{Message: "Invalid request body ", Success: false, Detail: err.Error()})
	}

	vErr := validateAdapter.ValidateData(&requestBody)
	if vErr != nil {
		return c.Status(fiber.StatusBadRequest).JSON(Response{Errors: vErr, Success: false, Detail: vErr})
	}

	scheduleTime := requestBody.ScheduleTime
	scheduleBuyTime := scheduleTime.Add(-time.Second * 30) // subtract 1 minute for ScheduleBuyTime
	scheduleSellTime := scheduleTime.Add(time.Minute * 1)  // add 15 minutes for ScheduleSellTime

	order = models.Order{
		Symbol:           requestBody.Symbol,
		ScheduleTime:     scheduleTime,
		ScheduleBuyTime:  &scheduleBuyTime,  // start the buy
		ScheduleSellTime: &scheduleSellTime, // start the sell
		Price:            requestBody.Price,
	}

	err := db.WithContext(ctx).Model(&models.Order{}).Create(&order).Error
	if err != nil {
		return c.Status(400).JSON(Response{Errors: err.Error(), Success: false, Detail: err.Error()})
	}

	// make the schedule
	order.ScheduleBuyScheduler(ctx, db)
	order.ScheduleSellScheduler(ctx, db)

	return c.Status(200).JSON(order)
}
