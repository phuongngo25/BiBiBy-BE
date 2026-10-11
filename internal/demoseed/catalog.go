// Package demoseed generates a realistic, deterministic history (meals, water,
// gym sessions and other activities) for a fixed set of demo accounts.
package demoseed

// FoodChoice references a VFA dish seeded by cmd/seeder (foods.code =
// "DISH-" + VFA code). VFA dish nutrition is stored per serving (the whole
// dish), so ServingGrams is the weight of that one serving.
type FoodChoice struct {
	Code         string
	ServingGrams float64
}

// Meal slots, matching the backend meal_type values the app sends.
const (
	MealBreakfast = "breakfast"
	MealLunch     = "lunch"
	MealDinner    = "dinner"
	MealSnack     = "snack"
)

var (
	breakfastFoods = []FoodChoice{
		{"HAN-113089", 500}, // Phở bò chín
		{"HAN-113091", 500}, // Phở bò tái
		{"HAN-111022", 500}, // Phở gà
		{"VPF-000220", 450}, // Bún bò Nam Bộ
		{"VPF-000219", 550}, // Bún bò giò heo (Huế)
		{"HAP-211090", 500}, // Bún riêu cua
		{"HAN-121061", 300}, // Bánh cuốn
		{"HAP-211009", 300}, // Bánh cuốn thịt
		{"VPF-000310", 150}, // Bánh mỳ pate
		{"VPF-000309", 180}, // Bánh mỳ ốp la
		{"HAP-223008", 180}, // Bánh mỳ pate trứng
		{"VPF-000363", 200}, // Xôi xéo
		{"VPF-000353", 250}, // Xôi gà
		{"HAP-221031", 400}, // Cháo thịt
		{"VPF-000243", 400}, // Cháo gà
		{"VPF-000255", 500}, // Hủ tiếu nước
	}
	// Extra breakfast drink/side, added when the breakfast budget allows.
	breakfastSides = []FoodChoice{
		{"VPF-000394", 180}, // Sữa tươi tiệt trùng không đường TH True Milk
		{"VPF-000390", 200}, // Sữa đậu nành
		{"VPF-000375", 50},  // Trứng gà công nghiệp luộc
	}
	lunchFoods = []FoodChoice{
		{"HAP-211026", 450}, // Cơm suất (ba chỉ kho)
		{"HAP-213034", 450}, // Cơm suất (sườn, đậu, chả lá lốt)
		{"HAP-213002", 450}, // Cơm suất (thịt gà, lợn, trứng)
		{"VPF-000011", 450}, // Cơm suất văn phòng (nạc vai, su hào)
		{"VPF-000012", 450}, // Cơm suất văn phòng (ba chỉ, đậu phụ)
		{"VPF-000013", 400}, // Cơm sườn
		{"HAN-121017", 400}, // Cơm rang dưa bò
		{"VPF-000009", 400}, // Cơm rang thập cẩm
		{"HAP-213032", 450}, // Bún chả
		{"VPF-000222", 450}, // Bún chả cá
		{"HAP-211018", 500}, // Phở bò tái
		{"VPF-000257", 450}, // Miến gà
		{"VPF-000216", 500}, // Bánh canh thịt heo
	}
	dinnerRice  = FoodChoice{"VPF-000006", 200} // Cơm tẻ miệng bát
	dinnerMains = []FoodChoice{
		{"VPF-000053", 200}, // Thịt bò kho
		{"VPF-000056", 150}, // Thịt lợn ba chỉ kho tàu
		{"VPF-000059", 120}, // Thịt lợn thăn rim
		{"VPF-000049", 200}, // Đậu phụ kho thịt
		{"VPF-000123", 200}, // Đậu phụ nhồi thịt sốt cà chua
		{"VPF-000045", 150}, // Cá thu kho
		{"VPF-000042", 180}, // Cá Ngừ kho dứa
		{"VPF-000116", 150}, // Cá thu rán
		{"VPF-000193", 200}, // Thịt gà xào sả
		{"VPF-000185", 200}, // Thịt bò xào hành tây, cà chua
		{"VPF-000181", 200}, // Sườn xào chua ngọt
		{"VPF-000105", 200}, // Gà nướng
		{"VPF-000146", 150}, // Trứng đúc thịt rán
		{"VPF-000144", 150}, // Tôm sốt cà chua
	}
	dinnerSides = []FoodChoice{
		{"VPF-000179", 200}, // Rau muống xào
		{"VPF-000173", 200}, // Rau cải bắp xào
		{"VPF-000159", 150}, // Đậu cove xào
		{"VPF-000086", 200}, // Rau muống luộc
		{"VPF-000033", 300}, // Canh rau ngót nấu thịt
		{"VPF-000025", 300}, // Canh cải xanh thịt nạc
		{"VPF-000020", 300}, // Canh bí đỏ thịt nạc
		{"VPF-000029", 300}, // Canh đu đủ xanh, thịt nạc
	}
	snackFoods = []FoodChoice{
		{"VPF-000401", 120}, // Chuối tiêu
		{"VPF-000398", 200}, // Cam
		{"VPF-000423", 180}, // Táo tây
		{"VPF-000427", 200}, // Xoài
		{"VPF-000424", 250}, // Thanh long
		{"VPF-000417", 200}, // Ổi
		{"VPF-000397", 250}, // Bưởi
		{"VPF-000386", 100}, // Sữa chua có đường Ba Vì
		{"VPF-000387", 100}, // Sữa chua không đường TH True Yogurt
		{"VPF-000394", 180}, // Sữa tươi TH True Milk không đường
		{"VPF-000366", 250}, // Chè đỗ xanh
	}
)

// AllFoodCodes lists every VFA code the generator may log.
func AllFoodCodes() []string {
	var codes []string
	for _, list := range [][]FoodChoice{breakfastFoods, breakfastSides, lunchFoods, {dinnerRice}, dinnerMains, dinnerSides, snackFoods} {
		for _, f := range list {
			codes = append(codes, f.Code)
		}
	}
	return codes
}

// Activity is one loggable workout. IDs match what the app sends to
// POST /workouts/log:
//   - Gym: bundled local catalog IDs (local-*), logged without met_value, so
//     the backend's default MET 5.0 applies.
//   - Other activities: the Compendium codes, names and METs from the app's
//     own activity list (Nutrix physical_activity_data_source.dart).
type Activity struct {
	ID   string
	Name string
	MET  float64
}

const gymMET = 5.0

var (
	exPushUp        = Activity{"local-chest-push-up", "Push-up", gymMET}
	exBenchPress    = Activity{"local-chest-bench-press", "Bench Press", gymMET}
	exRow           = Activity{"local-back-row", "Bent-over Row", gymMET}
	exLatPulldown   = Activity{"local-back-lat-pulldown", "Lat Pulldown", gymMET}
	exSquat         = Activity{"local-legs-squat", "Squat", gymMET}
	exLunge         = Activity{"local-legs-lunge", "Forward Lunge", gymMET}
	exShoulderPress = Activity{"local-shoulders-press", "Shoulder Press", gymMET}
	exLateralRaise  = Activity{"local-shoulders-lateral-raise", "Lateral Raise", gymMET}
	exCurl          = Activity{"local-arms-curl", "Biceps Curl", gymMET}
	exTricepsExt    = Activity{"local-arms-triceps-extension", "Overhead Triceps Extension", gymMET}
	exPlank         = Activity{"local-core-plank", "Plank", gymMET}
	exDeadBug       = Activity{"local-core-dead-bug", "Dead Bug", gymMET}

	gymPush  = []Activity{exBenchPress, exPushUp, exShoulderPress, exLateralRaise, exTricepsExt}
	gymPull  = []Activity{exRow, exLatPulldown, exCurl, exPlank}
	gymLegs  = []Activity{exSquat, exLunge, exPlank, exDeadBug}
	gymUpper = []Activity{exBenchPress, exRow, exShoulderPress, exCurl, exTricepsExt}
	gymFull  = []Activity{exSquat, exBenchPress, exRow, exShoulderPress, exPlank}

	actRunning    = Activity{"12150", "running", 8.0}
	actJogging    = Activity{"12020", "jogging", 7.0}
	actCycling    = Activity{"01015", "bicycling", 7.5}
	actStationary = Activity{"02010", "bicycling, stationary", 7.5}
	actWalking    = Activity{"17160", "walking", 3.5}
	actSwimming   = Activity{"18350", "swimming", 3.5}
	actBadminton  = Activity{"15030", "badminton", 5.5}
	actSoccer     = Activity{"15610", "soccer", 7.0}
	actBasketball = Activity{"15055", "basketball", 6.0}
	actTennis     = Activity{"15675", "tennis", 7.3}
	actTableTen   = Activity{"15660", "table tennis", 4.0}
	actVolleyball = Activity{"15710", "volleyball", 4.0}
	actAerobic    = Activity{"03015", "aerobic", 7.3}
	actRope       = Activity{"02068", "rope skipping", 12.3}
	actTaiChi     = Activity{"15670", "tai chi, qi gong", 3.0}
)
