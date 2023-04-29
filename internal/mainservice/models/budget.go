package models

type GeneralInfo struct {
	Age  int    `json:"age" validate:"required"`
	City string `json:"city" validate:"required"`
}

type Income struct {
	Fixed       int `json:"fixed" validate:"required"`
	Variable    int `json:"variable"`
	Investments int `json:"investments"`
	Other       int `json:"other" validate:"required"`
}

type Lifestyle struct {
	FoodExpense              int    `json:"foodExpense" validate:"required"`
	Diet                     string `json:"diet"`
	MostFrequentFood         string `json:"mostFrequentFood"`
	GroceryExpense           int    `json:"groceryExpense"`
	CookVsPrepared           string `json:"cookVsPrepared"`
	EatingOutFrequency       int    `json:"eatingOutFrequency" validate:"required"`
	EatingOutExpense         int    `json:"eatingOutExpense"`
	RecreationalOutFrequency int    `json:"recreationalOutFrequency" validate:"required"`
	RecreationalOutExpense   int    `json:"recreationalOutExpense"`
}

type Streaming struct {
	Accounts    int      `json:"accounts"`
	Expense     int      `json:"expense"`
	Preferences []string `json:"preferences"`
}

type Personal struct {
	Dependents    string   `json:"dependents" validate:"required"`
	MaritalStatus string   `json:"maritalStatus"`
	Pets          []string `json:"pets"`
	Gender        string   `json:"gender"`
}

type Assets struct {
	OwnHouse        bool        `json:"ownHouse" validate:"required"`
	RentExpense     int         `json:"rentExpense" validate:"required"`
	Mortgage        int         `json:"mortgage"`
	OwnCar          bool        `json:"ownCar"`
	OtherProperties interface{} `json:"otherProperties"`
}

type FinancialGoals struct {
	ShortTerm  string `json:"shortTerm" validate:"required"`
	MediumTerm string `json:"mediumTerm" validate:"required"`
	LongTerm   string `json:"longTerm" validate:"required"`
}

type AdditionalInfo struct {
	OtherServicesExpense int    `json:"otherServicesExpense"`
	Comments             string `json:"comments"`
}

type Hobbies struct {
	Main    []string    `json:"main"`
	Expense int         `json:"expense"`
	Travel  TravelHobby `json:"travel"`
}

type TravelHobby struct {
	Like           bool `json:"like"`
	Frequency      int  `json:"frequency"`
	AverageExpense int  `json:"averageExpense"`
}

type Request struct {
	GeneralInfo    GeneralInfo    `json:"generalInfo"`
	Income         Income         `json:"income"`
	Lifestyle      Lifestyle      `json:"lifestyle"`
	Streaming      Streaming      `json:"streaming"`
	Personal       Personal       `json:"personal"`
	Assets         Assets         `json:"assets"`
	FinancialGoals FinancialGoals `json:"financialGoals"`
	AdditionalInfo AdditionalInfo `json:"additionalInfo"`
	Hobbies        Hobbies        `json:"hobbies"`
}
