package usecase

import (
	"strings"

	"nutrix-backend/internal/domain"
)

type localExercise struct {
	ID            string
	Name          string
	Equipment     string
	BodyParts     []string
	TargetMuscles []string
	Instructions  []string
}

var localExerciseCatalog = []localExercise{
	{
		ID:            "local-chest-push-up",
		Name:          "Push-up",
		Equipment:     "Bodyweight",
		BodyParts:     []string{"chest"},
		TargetMuscles: []string{"pectorals", "triceps", "front delts"},
		Instructions: []string{
			"Start in a high plank with hands just wider than shoulder width.",
			"Lower your chest toward the floor while keeping your body in a straight line.",
			"Press back up until your elbows are extended.",
		},
	},
	{
		ID:            "local-chest-bench-press",
		Name:          "Bench Press",
		Equipment:     "Barbell",
		BodyParts:     []string{"chest"},
		TargetMuscles: []string{"pectorals", "triceps", "front delts"},
		Instructions: []string{
			"Lie on a bench with feet planted and grip the bar slightly wider than shoulder width.",
			"Lower the bar under control to the mid chest.",
			"Press the bar upward until your arms are extended.",
		},
	},
	{
		ID:            "local-back-row",
		Name:          "Bent-over Row",
		Equipment:     "Barbell or dumbbells",
		BodyParts:     []string{"back"},
		TargetMuscles: []string{"lats", "rhomboids", "rear delts"},
		Instructions: []string{
			"Hinge at the hips with a neutral spine.",
			"Pull the weight toward your lower ribs.",
			"Lower the weight under control and repeat.",
		},
	},
	{
		ID:            "local-back-lat-pulldown",
		Name:          "Lat Pulldown",
		Equipment:     "Cable machine",
		BodyParts:     []string{"back"},
		TargetMuscles: []string{"lats", "biceps"},
		Instructions: []string{
			"Sit tall and grip the bar wider than shoulder width.",
			"Pull the bar toward your upper chest while driving elbows down.",
			"Return the bar upward with control.",
		},
	},
	{
		ID:            "local-legs-squat",
		Name:          "Squat",
		Equipment:     "Bodyweight or barbell",
		BodyParts:     []string{"legs"},
		TargetMuscles: []string{"quads", "glutes", "hamstrings"},
		Instructions: []string{
			"Stand with feet about shoulder width apart.",
			"Sit hips down and back while keeping your chest lifted.",
			"Drive through your feet to stand back up.",
		},
	},
	{
		ID:            "local-legs-lunge",
		Name:          "Forward Lunge",
		Equipment:     "Bodyweight or dumbbells",
		BodyParts:     []string{"legs"},
		TargetMuscles: []string{"quads", "glutes", "hamstrings"},
		Instructions: []string{
			"Step one foot forward and lower until both knees are bent.",
			"Keep the front knee tracking over the toes.",
			"Push through the front foot to return to standing.",
		},
	},
	{
		ID:            "local-shoulders-press",
		Name:          "Shoulder Press",
		Equipment:     "Dumbbells",
		BodyParts:     []string{"shoulders"},
		TargetMuscles: []string{"delts", "triceps"},
		Instructions: []string{
			"Hold dumbbells at shoulder height with elbows slightly forward.",
			"Press the weights overhead without leaning back.",
			"Lower to shoulder height with control.",
		},
	},
	{
		ID:            "local-shoulders-lateral-raise",
		Name:          "Lateral Raise",
		Equipment:     "Dumbbells",
		BodyParts:     []string{"shoulders"},
		TargetMuscles: []string{"side delts"},
		Instructions: []string{
			"Stand tall with dumbbells by your sides.",
			"Raise the weights out to the sides until near shoulder height.",
			"Lower slowly without swinging.",
		},
	},
	{
		ID:            "local-arms-curl",
		Name:          "Biceps Curl",
		Equipment:     "Dumbbells",
		BodyParts:     []string{"arms"},
		TargetMuscles: []string{"biceps"},
		Instructions: []string{
			"Stand tall with dumbbells at your sides.",
			"Curl the weights up while keeping elbows close to your torso.",
			"Lower slowly to the starting position.",
		},
	},
	{
		ID:            "local-arms-triceps-extension",
		Name:          "Overhead Triceps Extension",
		Equipment:     "Dumbbell",
		BodyParts:     []string{"arms"},
		TargetMuscles: []string{"triceps"},
		Instructions: []string{
			"Hold a dumbbell overhead with both hands.",
			"Bend your elbows to lower the weight behind your head.",
			"Extend your elbows to raise the weight back overhead.",
		},
	},
	{
		ID:            "local-core-plank",
		Name:          "Plank",
		Equipment:     "Bodyweight",
		BodyParts:     []string{"core"},
		TargetMuscles: []string{"abs", "obliques"},
		Instructions: []string{
			"Set your elbows under your shoulders and extend your legs.",
			"Brace your core and keep your body in a straight line.",
			"Hold the position while breathing steadily.",
		},
	},
	{
		ID:            "local-core-dead-bug",
		Name:          "Dead Bug",
		Equipment:     "Bodyweight",
		BodyParts:     []string{"core"},
		TargetMuscles: []string{"abs", "deep core"},
		Instructions: []string{
			"Lie on your back with arms up and knees bent over hips.",
			"Slowly lower the opposite arm and leg toward the floor.",
			"Return to the start and alternate sides.",
		},
	},
}

func localExercisesByBodyPart(bodyPart string) []domain.ExerciseListItem {
	normalized := normalizeExerciseGroup(bodyPart)
	if normalized == "" {
		return nil
	}

	items := make([]domain.ExerciseListItem, 0)
	for _, exercise := range localExerciseCatalog {
		if !containsNormalized(exercise.BodyParts, normalized) &&
			!containsNormalized(exercise.TargetMuscles, normalized) {
			continue
		}

		items = append(items, domain.ExerciseListItem{
			ExerciseID:    exercise.ID,
			Name:          exercise.Name,
			Equipment:     exercise.Equipment,
			BodyParts:     exercise.BodyParts,
			TargetMuscles: exercise.TargetMuscles,
		})
	}

	return items
}

func localExerciseDetailByID(id string) (*domain.ExerciseDetail, bool) {
	for _, exercise := range localExerciseCatalog {
		if exercise.ID != id {
			continue
		}

		return &domain.ExerciseDetail{
			ExerciseID:   exercise.ID,
			Name:         exercise.Name,
			Instructions: exercise.Instructions,
			Equipments:   []string{exercise.Equipment},
		}, true
	}

	return nil, false
}

func normalizeExerciseGroup(value string) string {
	value = strings.ToLower(strings.TrimSpace(value))
	value = strings.ReplaceAll(value, "_", " ")
	value = strings.ReplaceAll(value, "-", " ")

	switch value {
	case "chest", "pectoral", "pectorals":
		return "chest"
	case "back", "lats", "lat", "upper back":
		return "back"
	case "leg", "legs", "quads", "quadriceps", "hamstrings", "glutes":
		return "legs"
	case "shoulder", "shoulders", "delts", "deltoids":
		return "shoulders"
	case "arm", "arms", "biceps", "triceps":
		return "arms"
	case "core", "abs", "abdominals", "obliques":
		return "core"
	default:
		return value
	}
}

func containsNormalized(values []string, target string) bool {
	for _, value := range values {
		if normalizeExerciseGroup(value) == target {
			return true
		}
	}
	return false
}
