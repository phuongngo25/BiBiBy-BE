package infrastructure

import (
	"encoding/json"
	"reflect"
	"strings"
	"testing"

	"nutrix-backend/internal/domain"
	commonpb "nutrix-backend/internal/infrastructure/grpc/pb/commonpb"
	pb "nutrix-backend/internal/infrastructure/grpc/pb/intelligencepb"
)

func mappingF64(v float64) *float64 { return &v }

const clientMassJSON = `{"candidate":{"meal_id":"m","food_ids":["F"],` +
	`"food_masses":[{"food_id":"F","user_entered_mass_g":400,"actual_mass_g":180,"actual_mass_source":"cv_estimated"}],` +
	`"daily_intake":{"present":true,"items":[]}}}`

// TestMapping_JSONBindDropsClientDailyIntake: the REST body binds food_masses
// with presence intact, and a client-supplied daily_intake is never bound
// (T-10-16: a client cannot inject today's intake).
func TestMapping_JSONBindDropsClientDailyIntake(t *testing.T) {
	var req domain.AnalyzeMealRequest
	if err := json.Unmarshal([]byte(clientMassJSON), &req); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if req.Candidate.DailyIntake != nil {
		t.Fatalf("client daily_intake must not bind, got %+v", req.Candidate.DailyIntake)
	}
	if len(req.Candidate.FoodMasses) != 1 {
		t.Fatalf("expected one food mass, got %+v", req.Candidate.FoodMasses)
	}
	m := req.Candidate.FoodMasses[0]
	if m.FoodID != "F" || m.UserEnteredMassG == nil || *m.UserEnteredMassG != 400 ||
		m.ActualMassG == nil || *m.ActualMassG != 180 || m.ActualMassSource != "cv_estimated" {
		t.Fatalf("unexpected bound food mass: %+v", m)
	}
	if m.EstimatedServingG != nil {
		t.Fatalf("absent estimated_serving_g must stay nil, got %v", *m.EstimatedServingG)
	}
}

func TestMapping_RequestFoodMassesSeparateFields(t *testing.T) {
	var req domain.AnalyzeMealRequest
	if err := json.Unmarshal([]byte(clientMassJSON), &req); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}

	got := buildPbCandidateMeal(req.Candidate)
	if len(got.FoodMasses) != 1 {
		t.Fatalf("expected one pb food mass, got %+v", got.FoodMasses)
	}
	m := got.FoodMasses[0]
	if m.GetFoodId() != "F" {
		t.Fatalf("unexpected food id %q", m.GetFoodId())
	}
	if m.UserEnteredMassG == nil || *m.UserEnteredMassG != 400 {
		t.Fatalf("expected user_entered_mass_g 400, got %v", m.UserEnteredMassG)
	}
	if m.ActualMassG == nil || *m.ActualMassG != 180 {
		t.Fatalf("expected actual_mass_g 180, got %v", m.ActualMassG)
	}
	if m.ActualMassSource != pb.MassSource_MASS_SOURCE_CV_ESTIMATED {
		t.Fatalf("expected MASS_SOURCE_CV_ESTIMATED, got %v", m.ActualMassSource)
	}
	if m.EstimatedServingG != nil {
		t.Fatalf("expected estimated_serving_g unset, got %v", *m.EstimatedServingG)
	}
	if got.DailyIntake != nil {
		t.Fatalf("nil domain DailyIntake must stay unset on the wire")
	}

	// Fresh pointers: mutating the domain value after mapping must not leak.
	*req.Candidate.FoodMasses[0].UserEnteredMassG = 1
	*req.Candidate.FoodMasses[0].ActualMassG = 2
	if *m.UserEnteredMassG != 400 || *m.ActualMassG != 180 {
		t.Fatalf("pb food mass aliases the domain value: %v / %v", *m.UserEnteredMassG, *m.ActualMassG)
	}
}

func TestMapping_ActualMassSourceEnum(t *testing.T) {
	cases := map[string]pb.MassSource{
		"cv_estimated":      pb.MassSource_MASS_SOURCE_CV_ESTIMATED,
		"measured":          pb.MassSource_MASS_SOURCE_MEASURED,
		"":                  pb.MassSource_MASS_SOURCE_UNSPECIFIED,
		"user_entered":      pb.MassSource_MASS_SOURCE_UNSPECIFIED,
		"estimated_serving": pb.MassSource_MASS_SOURCE_UNSPECIFIED,
		"bogus":             pb.MassSource_MASS_SOURCE_UNSPECIFIED,
	}
	for source, want := range cases {
		got := buildPbCandidateMeal(domain.CandidateMeal{FoodMasses: []domain.FoodMassInput{{
			FoodID: "F", ActualMassG: mappingF64(100), ActualMassSource: source,
		}}})
		if got.FoodMasses[0].ActualMassSource != want {
			t.Errorf("actual_mass_source %q: want %v, got %v", source, want, got.FoodMasses[0].ActualMassSource)
		}
	}
}

func TestMapping_ResponseMassResolutions(t *testing.T) {
	resp := &pb.AnalyzeMealResponse{
		Status: pb.MealApprovalStatus_WARNING,
		MassResolutions: []*pb.MassResolution{
			{
				FoodId:                  "F",
				MassG:                   mappingF64(180),
				MassSource:              pb.MassSource_MASS_SOURCE_CV_ESTIMATED,
				Hc04Approved:            false,
				DecisionAuthority:       false,
				AuthoritativeMassG:      mappingF64(400),
				AuthoritativeMassSource: pb.MassSource_MASS_SOURCE_USER_ENTERED,
				Note:                    "CV mass is unverified",
			},
			{FoodId: "G", MassSource: pb.MassSource_MASS_SOURCE_UNSPECIFIED},
		},
	}

	got := mapPbAnalyzeMealResponse(resp)
	if len(got.MassResolutions) != 2 {
		t.Fatalf("expected two mass resolutions, got %+v", got.MassResolutions)
	}
	r := got.MassResolutions[0]
	if r.FoodID != "F" || r.MassSource != "cv_estimated" || r.AuthoritativeMassSource != "user_entered" {
		t.Fatalf("unexpected mass resolution: %+v", r)
	}
	if r.MassG == nil || *r.MassG != 180 || r.AuthoritativeMassG == nil || *r.AuthoritativeMassG != 400 {
		t.Fatalf("expected mass 180 / authoritative 400, got %v / %v", r.MassG, r.AuthoritativeMassG)
	}
	if r.HC04Approved || r.DecisionAuthority || r.Note != "CV mass is unverified" {
		t.Fatalf("unexpected provenance flags: %+v", r)
	}
	*resp.MassResolutions[0].MassG = 1
	if *r.MassG != 180 {
		t.Fatalf("domain mass resolution aliases the pb value")
	}

	unset := got.MassResolutions[1]
	if unset.MassG != nil || unset.AuthoritativeMassG != nil {
		t.Fatalf("unset masses must stay nil, got %+v", unset)
	}
	if unset.MassSource != "" || unset.AuthoritativeMassSource != "" {
		t.Fatalf("UNSPECIFIED must map to empty source, got %+v", unset)
	}

	for source, want := range map[pb.MassSource]string{
		pb.MassSource_MASS_SOURCE_CV_ESTIMATED:      "cv_estimated",
		pb.MassSource_MASS_SOURCE_USER_ENTERED:      "user_entered",
		pb.MassSource_MASS_SOURCE_ESTIMATED_SERVING: "estimated_serving",
		pb.MassSource_MASS_SOURCE_MEASURED:          "measured",
		pb.MassSource_MASS_SOURCE_UNSPECIFIED:       "",
	} {
		m := mapPbAnalyzeMealResponse(&pb.AnalyzeMealResponse{MassResolutions: []*pb.MassResolution{{MassSource: source}}})
		if m.MassResolutions[0].MassSource != want {
			t.Errorf("mass_source %v: want %q, got %q", source, want, m.MassResolutions[0].MassSource)
		}
	}
}

// TestMapping_LegacyResponseShape: a pre-Phase-10 response maps exactly as
// before and serializes without any of the new keys.
func TestMapping_LegacyResponseShape(t *testing.T) {
	resp := &pb.AnalyzeMealResponse{
		Status: pb.MealApprovalStatus_WARNING,
		Score:  &pb.MealScore{SafetyScore: 0.5, MacroScore: 1, MicronutrientScore: 0.75, ConstraintScore: 1},
		Violations: []*pb.MealViolation{{
			ViolationType:    "hypertension",
			Description:      "High sodium food for hypertension",
			Severity:         commonpb.RiskLevel_RISK_LEVEL_HIGH,
			OffendingFoodIds: []string{"F"},
		}},
		Fixes: []*pb.MealFixSuggestion{{
			Title:       "Swap soup",
			Replacement: &pb.CandidateMeal{MealId: "r", FoodIds: []string{"G"}, MealType: "lunch"},
			Impact:      &pb.MealFixImpact{SafetyDelta: 1},
		}},
		Enrichment: &pb.MealEnrichment{
			DishName:              "Pho",
			EstimatedServingSizeG: 500,
			Ingredients:           []*pb.MealIngredientDetail{{Name: "noodle", AmountG: 200}},
			Source:                "llm",
			Confidence:            0.9,
		},
	}

	want := &domain.AnalyzeMealResponse{
		Status: "WARNING",
		Score:  domain.MealScore{SafetyScore: 0.5, MacroScore: 1, MicronutrientScore: 0.75, ConstraintScore: 1},
		Violations: []domain.MealViolation{{
			ViolationType:    "hypertension",
			Description:      "High sodium food for hypertension",
			Severity:         "RISK_LEVEL_HIGH",
			OffendingFoodIDs: []string{"F"},
		}},
		Fixes: []domain.MealFixSuggestion{{
			Title:       "Swap soup",
			Replacement: domain.CandidateMeal{MealID: "r", FoodIDs: []string{"G"}, MealType: "lunch"},
			Impact:      domain.MealFixImpact{SafetyDelta: 1},
		}},
		Enrichment: &domain.MealEnrichment{
			DishName:              "Pho",
			EstimatedTotalWeightG: 500,
			Ingredients:           []domain.MealIngredientEstimate{{Name: "noodle", WeightG: 200}},
			Source:                "llm",
			Confidence:            0.9,
		},
	}

	got := mapPbAnalyzeMealResponse(resp)
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("legacy mapping changed:\n got %+v\nwant %+v", got, want)
	}

	raw, err := json.Marshal(got)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	for _, key := range []string{"nutrientResults", "massResolutions", "nutrient_id", "projected_mg", "threshold_mg", "food_masses", "daily_intake"} {
		if strings.Contains(string(raw), `"`+key+`"`) {
			t.Errorf("legacy JSON must not contain %q: %s", key, raw)
		}
	}

	if empty := mapPbAnalyzeMealResponse(nil); empty == nil {
		t.Fatalf("nil response must map to a non-nil empty response")
	}
}

// TestMapping_NoRequestAuthorityFields (T-10-18): no request-side type, domain
// or generated, can carry an authority or HC-04 decision. Authority lives on
// the response-only MassResolution and is computed by AI_server alone.
func TestMapping_NoRequestAuthorityFields(t *testing.T) {
	roots := []reflect.Type{
		reflect.TypeOf(domain.FoodMassInput{}),
		reflect.TypeOf(domain.CandidateMeal{}),
		reflect.TypeOf(domain.ConsumedFoodItem{}),
		reflect.TypeOf(domain.DailyIntakeContext{}),
		reflect.TypeOf(domain.AnalyzeMealRequest{}),
		reflect.TypeOf(domain.UserMedicalContext{}),
		reflect.TypeOf(pb.FoodMassInput{}),
		reflect.TypeOf(pb.CandidateMeal{}),
		reflect.TypeOf(pb.ConsumedFoodItem{}),
		reflect.TypeOf(pb.DailyIntakeContext{}),
		reflect.TypeOf(pb.AnalyzeMealRequest{}),
		reflect.TypeOf(pb.UserMedicalContext{}),
	}
	seen := map[reflect.Type]bool{}
	var walk func(reflect.Type)
	walk = func(typ reflect.Type) {
		for typ.Kind() == reflect.Pointer || typ.Kind() == reflect.Slice {
			typ = typ.Elem()
		}
		if typ.Kind() != reflect.Struct || seen[typ] {
			return
		}
		seen[typ] = true
		for i := 0; i < typ.NumField(); i++ {
			field := typ.Field(i)
			if !field.IsExported() {
				continue // protoimpl state / sizeCache / unknownFields
			}
			for _, label := range []string{field.Name, field.Tag.Get("json"), field.Tag.Get("protobuf")} {
				lower := strings.ToLower(label)
				if strings.Contains(lower, "authority") || strings.Contains(lower, "hc04") {
					t.Errorf("%s.%s is an authority-like request field (%q)", typ, field.Name, label)
				}
			}
			walk(field.Type)
		}
	}
	for _, root := range roots {
		walk(root)
	}
}

// clientMedicalJSON is a REST body that tries to supply its own medical
// context, under both the snake_case and the Go field names (plan 10-04).
const clientMedicalJSON = `{"candidate":{"meal_id":"m","food_ids":["F"],"meal_type":"lunch"},` +
	`"medical_context":{"present":true,"disease_ids":[]},` +
	`"MedicalContext":{"Present":true,"DiseaseIDs":[]}}`

// TestMapping_JSONBindDropsClientMedicalContext (T-10-23): a client-supplied
// medical context is never bound, so a client cannot drop its own conditions.
func TestMapping_JSONBindDropsClientMedicalContext(t *testing.T) {
	var req domain.AnalyzeMealRequest
	if err := json.Unmarshal([]byte(clientMedicalJSON), &req); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if req.MedicalContext != nil {
		t.Fatalf("client medical context must not bind, got %+v", req.MedicalContext)
	}
	if req.Candidate.MealID != "m" || len(req.Candidate.FoodIDs) != 1 {
		t.Fatalf("candidate must still bind, got %+v", req.Candidate)
	}
}

func TestMapping_MedicalContextMapping(t *testing.T) {
	if got := buildPbMedicalContext(nil); got != nil {
		t.Fatalf("nil must map to nil (field unset), got %+v", got)
	}

	ids := []string{"hypertension", "ckd"}
	got := buildPbMedicalContext(&domain.UserMedicalContext{Present: true, DiseaseIDs: ids})
	if !got.GetPresent() || !reflect.DeepEqual(got.GetDiseaseIds(), []string{"hypertension", "ckd"}) {
		t.Fatalf("unexpected mapping %+v", got)
	}
	ids[0] = "mutated"
	if got.GetDiseaseIds()[0] != "hypertension" {
		t.Fatalf("pb ids must be a copy, got %q", got.GetDiseaseIds())
	}

	unknown := buildPbMedicalContext(&domain.UserMedicalContext{Present: false})
	if unknown == nil || unknown.GetPresent() || len(unknown.GetDiseaseIds()) != 0 {
		t.Fatalf("present=false must map to present=false with no ids, got %+v", unknown)
	}
}
