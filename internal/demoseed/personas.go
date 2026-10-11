package demoseed

import (
	"fmt"
	"time"

	"nutrix-backend/internal/domain"
)

// EmailDomain marks demo accounts. The seeder only ever touches users whose
// email ends with it, so real accounts are never modified.
const EmailDomain = "demo.bibiby.space"

// Session is one planned training slot in a persona's week.
type Session struct {
	Gym      []Activity // gym exercise pool; 3+ of them are performed
	Activity []Activity // otherwise: one activity picked from this list
	MinMin   int        // activity duration range (minutes)
	MaxMin   int
}

// Persona describes one demo user and their habits.
type Persona struct {
	Username      string
	FullName      string
	Gender        string // "male" | "female"
	BirthYear     int
	HeightCm      float64
	StartWeightKg float64
	WeightPerDay  float64 // kg/day trend over the window (negative = losing)
	ActivityLevel domain.ActivityLevel
	GoalType      domain.GoalType

	LogRate     float64 // probability the user logs anything on a given day
	CalorieMean float64 // eaten / target, mean
	CalorieSD   float64
	WaterMean   float64 // drunk / target, mean
	SnackRate   float64 // probability of an afternoon snack
	Adherence   float64 // probability a planned workout happens
	Morning     bool    // trains in the morning instead of the evening

	Week [7]*Session // indexed by time.Weekday; nil = rest
}

// Email is the persona's login email.
func (p Persona) Email() string { return p.Username + "@" + EmailDomain }

// Password is the persona's login password (satisfies the backend's
// 8+ chars / upper / lower / digit rule).
func (p Persona) Password(index int) string { return fmt.Sprintf("Bibiby@%02d26", index+1) }

func gym(pool []Activity) *Session { return &Session{Gym: pool} }

func act(min, max int, opts ...Activity) *Session {
	return &Session{Activity: opts, MinMin: min, MaxMin: max}
}

// Personas are the 20 demo accounts, with deliberately varied goals, sports
// and discipline so analytics, streaks and leaderboards look realistic.
var Personas = []Persona{
	{Username: "an.nguyen", FullName: "Nguyễn Văn An", Gender: "male", BirthYear: 1998, HeightCm: 172, StartWeightKg: 78, WeightPerDay: -0.05,
		ActivityLevel: domain.ActivityActive, GoalType: domain.GoalLoseWeight,
		LogRate: 0.97, CalorieMean: 1.0, CalorieSD: 0.08, WaterMean: 1.08, SnackRate: 0.4, Adherence: 0.92,
		Week: [7]*Session{time.Monday: gym(gymPush), time.Tuesday: gym(gymPull), time.Wednesday: act(25, 40, actRunning), time.Thursday: gym(gymLegs), time.Friday: gym(gymUpper), time.Saturday: act(40, 70, actRunning, actCycling)}},
	{Username: "binh.tran", FullName: "Trần Thị Bình", Gender: "female", BirthYear: 2000, HeightCm: 158, StartWeightKg: 56, WeightPerDay: -0.02,
		ActivityLevel: domain.ActivityLowActive, GoalType: domain.GoalLoseWeight,
		LogRate: 0.9, CalorieMean: 1.02, CalorieSD: 0.12, WaterMean: 0.95, SnackRate: 0.7, Adherence: 0.8, Morning: true,
		Week: [7]*Session{time.Monday: act(30, 45, actAerobic), time.Wednesday: act(30, 50, actWalking), time.Friday: act(30, 45, actAerobic), time.Sunday: act(45, 60, actBadminton)}},
	{Username: "cuong.le", FullName: "Lê Mạnh Cường", Gender: "male", BirthYear: 1995, HeightCm: 178, StartWeightKg: 70, WeightPerDay: 0.04,
		ActivityLevel: domain.ActivityVeryActive, GoalType: domain.GoalGainWeight,
		LogRate: 0.95, CalorieMean: 1.0, CalorieSD: 0.09, WaterMean: 1.05, SnackRate: 0.9, Adherence: 0.95,
		Week: [7]*Session{time.Monday: gym(gymPush), time.Tuesday: gym(gymPull), time.Wednesday: gym(gymLegs), time.Thursday: gym(gymPush), time.Friday: gym(gymPull), time.Saturday: gym(gymLegs)}},
	{Username: "dung.pham", FullName: "Phạm Thùy Dung", Gender: "female", BirthYear: 1997, HeightCm: 162, StartWeightKg: 52, WeightPerDay: 0,
		ActivityLevel: domain.ActivityActive, GoalType: domain.GoalMaintain,
		LogRate: 0.93, CalorieMean: 0.98, CalorieSD: 0.1, WaterMean: 1.1, SnackRate: 0.6, Adherence: 0.85, Morning: true,
		Week: [7]*Session{time.Monday: act(30, 45, actJogging), time.Tuesday: gym(gymFull), time.Thursday: act(30, 45, actJogging), time.Friday: gym(gymFull), time.Sunday: act(45, 60, actSwimming)}},
	{Username: "duc.hoang", FullName: "Hoàng Minh Đức", Gender: "male", BirthYear: 1990, HeightCm: 168, StartWeightKg: 85, WeightPerDay: -0.04,
		ActivityLevel: domain.ActivitySedentary, GoalType: domain.GoalLoseWeight,
		LogRate: 0.8, CalorieMean: 1.12, CalorieSD: 0.15, WaterMean: 0.8, SnackRate: 0.5, Adherence: 0.6,
		Week: [7]*Session{time.Tuesday: act(30, 45, actWalking), time.Thursday: act(30, 45, actWalking), time.Saturday: act(45, 60, actBadminton)}},
	{Username: "giang.vu", FullName: "Vũ Hương Giang", Gender: "female", BirthYear: 2002, HeightCm: 160, StartWeightKg: 48, WeightPerDay: 0.02,
		ActivityLevel: domain.ActivityLowActive, GoalType: domain.GoalGainWeight,
		LogRate: 0.85, CalorieMean: 0.95, CalorieSD: 0.12, WaterMean: 0.9, SnackRate: 0.85, Adherence: 0.75,
		Week: [7]*Session{time.Monday: gym(gymFull), time.Wednesday: gym(gymFull), time.Friday: gym(gymFull), time.Sunday: act(30, 45, actWalking)}},
	{Username: "hai.dang", FullName: "Đặng Thanh Hải", Gender: "male", BirthYear: 1993, HeightCm: 175, StartWeightKg: 74, WeightPerDay: 0,
		ActivityLevel: domain.ActivityVeryActive, GoalType: domain.GoalMaintain,
		LogRate: 0.96, CalorieMean: 1.0, CalorieSD: 0.07, WaterMean: 1.15, SnackRate: 0.6, Adherence: 0.93, Morning: true,
		Week: [7]*Session{time.Monday: act(40, 60, actRunning), time.Tuesday: act(45, 75, actCycling), time.Wednesday: act(40, 60, actRunning), time.Thursday: gym(gymFull), time.Friday: act(40, 60, actRunning), time.Saturday: act(60, 100, actRunning, actCycling), time.Sunday: act(30, 45, actSwimming)}},
	{Username: "ha.bui", FullName: "Bùi Thu Hà", Gender: "female", BirthYear: 1996, HeightCm: 155, StartWeightKg: 60, WeightPerDay: -0.035,
		ActivityLevel: domain.ActivityLowActive, GoalType: domain.GoalLoseWeight,
		LogRate: 0.92, CalorieMean: 1.0, CalorieSD: 0.1, WaterMean: 1.0, SnackRate: 0.5, Adherence: 0.85,
		Week: [7]*Session{time.Monday: act(30, 45, actAerobic), time.Tuesday: act(20, 30, actRope), time.Thursday: act(30, 45, actAerobic), time.Saturday: act(40, 60, actWalking, actSwimming)}},
	{Username: "hieu.do", FullName: "Đỗ Trung Hiếu", Gender: "male", BirthYear: 2001, HeightCm: 180, StartWeightKg: 68, WeightPerDay: 0.035,
		ActivityLevel: domain.ActivityActive, GoalType: domain.GoalGainWeight,
		LogRate: 0.88, CalorieMean: 0.97, CalorieSD: 0.1, WaterMean: 1.0, SnackRate: 0.8, Adherence: 0.85,
		Week: [7]*Session{time.Monday: gym(gymUpper), time.Tuesday: gym(gymLegs), time.Wednesday: act(60, 90, actSoccer), time.Thursday: gym(gymUpper), time.Friday: gym(gymLegs), time.Sunday: act(60, 90, actSoccer, actBasketball)}},
	{Username: "hoa.ngo", FullName: "Ngô Thị Hoa", Gender: "female", BirthYear: 1988, HeightCm: 157, StartWeightKg: 58, WeightPerDay: 0,
		ActivityLevel: domain.ActivitySedentary, GoalType: domain.GoalMaintain,
		LogRate: 0.75, CalorieMean: 1.05, CalorieSD: 0.14, WaterMean: 0.85, SnackRate: 0.6, Adherence: 0.7, Morning: true,
		Week: [7]*Session{time.Monday: act(30, 45, actTaiChi), time.Wednesday: act(30, 45, actWalking), time.Friday: act(30, 45, actTaiChi), time.Sunday: act(40, 60, actWalking)}},
	{Username: "huy.ly", FullName: "Lý Quang Huy", Gender: "male", BirthYear: 1999, HeightCm: 170, StartWeightKg: 72, WeightPerDay: -0.02,
		ActivityLevel: domain.ActivityActive, GoalType: domain.GoalLoseWeight,
		LogRate: 0.9, CalorieMean: 1.03, CalorieSD: 0.11, WaterMean: 0.98, SnackRate: 0.5, Adherence: 0.85,
		Week: [7]*Session{time.Monday: gym(gymPush), time.Tuesday: act(60, 90, actBadminton), time.Wednesday: gym(gymPull), time.Thursday: act(60, 90, actBadminton), time.Friday: gym(gymLegs), time.Saturday: act(60, 90, actBadminton)}},
	{Username: "khanh.duong", FullName: "Dương Ngọc Khánh", Gender: "female", BirthYear: 1994, HeightCm: 165, StartWeightKg: 55, WeightPerDay: 0,
		ActivityLevel: domain.ActivityActive, GoalType: domain.GoalMaintain,
		LogRate: 0.98, CalorieMean: 1.0, CalorieSD: 0.06, WaterMean: 1.12, SnackRate: 0.55, Adherence: 0.95, Morning: true,
		Week: [7]*Session{time.Monday: gym(gymUpper), time.Tuesday: act(30, 45, actJogging), time.Wednesday: gym(gymLegs), time.Thursday: act(45, 60, actSwimming), time.Friday: gym(gymFull), time.Saturday: act(45, 70, actJogging)}},
	{Username: "lam.trinh", FullName: "Trịnh Hoàng Lâm", Gender: "male", BirthYear: 1985, HeightCm: 169, StartWeightKg: 80, WeightPerDay: -0.03,
		ActivityLevel: domain.ActivityLowActive, GoalType: domain.GoalLoseWeight,
		LogRate: 0.82, CalorieMean: 1.08, CalorieSD: 0.13, WaterMean: 0.9, SnackRate: 0.35, Adherence: 0.7, Morning: true,
		Week: [7]*Session{time.Monday: act(30, 50, actWalking), time.Wednesday: act(45, 70, actTennis), time.Friday: act(30, 50, actWalking), time.Sunday: act(45, 70, actTennis)}},
	{Username: "linh.mai", FullName: "Mai Phương Linh", Gender: "female", BirthYear: 2003, HeightCm: 163, StartWeightKg: 50, WeightPerDay: 0,
		ActivityLevel: domain.ActivityLowActive, GoalType: domain.GoalMaintain,
		LogRate: 0.7, CalorieMean: 0.93, CalorieSD: 0.15, WaterMean: 0.8, SnackRate: 0.8, Adherence: 0.6,
		Week: [7]*Session{time.Tuesday: act(30, 45, actAerobic), time.Thursday: act(45, 60, actVolleyball), time.Saturday: act(30, 45, actAerobic)}},
	{Username: "long.phan", FullName: "Phan Bảo Long", Gender: "male", BirthYear: 1997, HeightCm: 176, StartWeightKg: 76, WeightPerDay: 0,
		ActivityLevel: domain.ActivityVeryActive, GoalType: domain.GoalMaintain,
		LogRate: 0.94, CalorieMean: 1.01, CalorieSD: 0.08, WaterMean: 1.1, SnackRate: 0.7, Adherence: 0.9,
		Week: [7]*Session{time.Monday: gym(gymPush), time.Tuesday: gym(gymPull), time.Wednesday: gym(gymLegs), time.Thursday: act(60, 90, actBasketball), time.Friday: gym(gymUpper), time.Saturday: act(60, 90, actBasketball, actSoccer)}},
	{Username: "mai.ho", FullName: "Hồ Thanh Mai", Gender: "female", BirthYear: 1992, HeightCm: 160, StartWeightKg: 63, WeightPerDay: -0.04,
		ActivityLevel: domain.ActivityActive, GoalType: domain.GoalLoseWeight,
		LogRate: 0.95, CalorieMean: 0.99, CalorieSD: 0.08, WaterMean: 1.05, SnackRate: 0.45, Adherence: 0.9, Morning: true,
		Week: [7]*Session{time.Monday: gym(gymFull), time.Tuesday: act(30, 45, actStationary), time.Wednesday: gym(gymFull), time.Thursday: act(20, 30, actRope), time.Friday: gym(gymFull), time.Saturday: act(45, 60, actSwimming)}},
	{Username: "nam.vo", FullName: "Võ Hoài Nam", Gender: "male", BirthYear: 2000, HeightCm: 173, StartWeightKg: 66, WeightPerDay: 0.03,
		ActivityLevel: domain.ActivityActive, GoalType: domain.GoalGainWeight,
		LogRate: 0.86, CalorieMean: 0.96, CalorieSD: 0.11, WaterMean: 0.95, SnackRate: 0.85, Adherence: 0.8,
		Week: [7]*Session{time.Monday: gym(gymPush), time.Tuesday: gym(gymPull), time.Thursday: gym(gymLegs), time.Saturday: act(60, 90, actSoccer)}},
	{Username: "ngoc.truong", FullName: "Trương Bảo Ngọc", Gender: "female", BirthYear: 1999, HeightCm: 168, StartWeightKg: 57, WeightPerDay: 0,
		ActivityLevel: domain.ActivityActive, GoalType: domain.GoalMaintain,
		LogRate: 0.92, CalorieMean: 1.0, CalorieSD: 0.09, WaterMean: 1.02, SnackRate: 0.6, Adherence: 0.85, Morning: true,
		Week: [7]*Session{time.Monday: act(30, 45, actJogging), time.Wednesday: act(45, 70, actCycling), time.Friday: act(30, 45, actJogging), time.Saturday: act(45, 60, actTableTen), time.Sunday: act(60, 90, actCycling)}},
	{Username: "phuc.dinh", FullName: "Đinh Hữu Phúc", Gender: "male", BirthYear: 1991, HeightCm: 171, StartWeightKg: 88, WeightPerDay: -0.06,
		ActivityLevel: domain.ActivityLowActive, GoalType: domain.GoalLoseWeight,
		LogRate: 0.93, CalorieMean: 0.98, CalorieSD: 0.09, WaterMean: 1.05, SnackRate: 0.3, Adherence: 0.88,
		Week: [7]*Session{time.Monday: gym(gymFull), time.Tuesday: act(40, 60, actWalking), time.Wednesday: gym(gymFull), time.Thursday: act(40, 60, actWalking), time.Friday: gym(gymFull), time.Saturday: act(30, 45, actStationary), time.Sunday: act(45, 60, actSwimming)}},
	{Username: "trang.lai", FullName: "Lại Minh Trang", Gender: "female", BirthYear: 2001, HeightCm: 154, StartWeightKg: 47, WeightPerDay: 0,
		ActivityLevel: domain.ActivitySedentary, GoalType: domain.GoalMaintain,
		LogRate: 0.65, CalorieMean: 0.9, CalorieSD: 0.16, WaterMean: 0.75, SnackRate: 0.75, Adherence: 0.5,
		Week: [7]*Session{time.Wednesday: act(30, 45, actWalking), time.Saturday: act(40, 60, actBadminton)}},
}
