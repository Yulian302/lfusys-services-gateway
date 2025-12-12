package types

type User struct {
	ID string `json:"id" dynamodbav:"id"`
	RegisterUser
}

type RegisterUser struct {
	Name     string `json:"name" dynamodbav:"name" binding:"required"`
	Email    string `json:"email" dynamodbav:"email" binding:"required,email"`
	Password string `json:"password" dynamodbav:"password" binding:"required,min=6"`
}

type LoginUser struct {
	Email    string `json:"email" dynamodbav:"email" binding:"required,email"`
	Password string `json:"password" dynamodbav:"password" binding:"required,min=6"`
}
