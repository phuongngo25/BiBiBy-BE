package domain

import "context"

// DRIElements represents exactly identical keys across RDA/AI, EAR, and UL.
// Pointers are strictly used to safely parse and reflect NULL values perfectly intact into JSONB.
type DRIElements struct {
	CalciumMg     *float64 `json:"calcium_mg"`
	ChromiumMcg   *float64 `json:"chromium_mcg"`
	CopperMcg     *float64 `json:"copper_mcg"`
	FluorideMg    *float64 `json:"fluoride_mg"`
	IodineMcg     *float64 `json:"iodine_mcg"`
	IronMg        *float64 `json:"iron_mg"`
	MagnesiumMg   *float64 `json:"magnesium_mg"`
	ManganeseMg   *float64 `json:"manganese_mg"`
	MolybdenumMcg *float64 `json:"molybdenum_mcg"`
	PhosphorusMg  *float64 `json:"phosphorus_mg"`
	PhosphorusG   *float64 `json:"phosphorus_g"` // specific to UL
	SeleniumMcg   *float64 `json:"selenium_mcg"`
	ZincMg        *float64 `json:"zinc_mg"`
	PotassiumMg   *float64 `json:"potassium_mg"`
	SodiumMg      *float64 `json:"sodium_mg"`
	SodiumG       *float64 `json:"sodium_g"` // specific to UL
	ChlorideG     *float64 `json:"chloride_g"`
	ArsenicMg     *float64 `json:"arsenic_mg"`
	BoronMg       *float64 `json:"boron_mg"`
	NickelMg      *float64 `json:"nickel_mg"`
	Silicon       *float64 `json:"silicon"`
	Sulfate       *float64 `json:"sulfate"`
	VanadiumMg    *float64 `json:"vanadium_mg"`
}

// DRIVitamins captures all vitamins across differing life stages.
type DRIVitamins struct {
	VitaminAMcg       *float64 `json:"vitamin_a_mcg"`
	VitaminCMg        *float64 `json:"vitamin_c_mg"`
	VitaminDMcg       *float64 `json:"vitamin_d_mcg"`
	VitaminEMg        *float64 `json:"vitamin_e_mg"`
	VitaminKMcg       *float64 `json:"vitamin_k_mcg"`
	ThiaminMg         *float64 `json:"thiamin_mg"`
	RiboflavinMg      *float64 `json:"riboflavin_mg"`
	NiacinMg          *float64 `json:"niacin_mg"`
	VitaminB6Mg       *float64 `json:"vitamin_b6_mg"`
	FolateMcg         *float64 `json:"folate_mcg"`
	VitaminB12Mcg     *float64 `json:"vitamin_b12_mcg"`
	PantothenicAcidMg *float64 `json:"pantothenic_acid_mg"`
	BiotinMcg         *float64 `json:"biotin_mcg"`
	CholineMg         *float64 `json:"choline_mg"`
	CholineG          *float64 `json:"choline_g"` // specific to UL
	Carotenoids       *float64 `json:"carotenoids"`
}

// DRIMacronutrients wraps core macro requirements and caloric breakdowns.
type DRIMacronutrients struct {
	TotalWaterL         *float64 `json:"total_water_l"`
	CarbohydrateG       *float64 `json:"carbohydrate_g"`
	TotalFiberG         *float64 `json:"total_fiber_g"`
	FatG                *float64 `json:"fat_g"`
	LinoleicAcidG       *float64 `json:"linoleic_acid_g"`
	AlphaLinolenicAcidG *float64 `json:"alpha_linolenic_acid_g"`
	ProteinG            *float64 `json:"protein_g"`
	ProteinGPerKg       *float64 `json:"protein_g_per_kg"` // specific to EAR
}

// DRIAmdr dictates macronutrient distribution ranges, normally strings like "20-35" or null.
type DRIAmdr struct {
	FatPercent          *string `json:"fat_percent"`
	N6Percent           *string `json:"n_6_percent"`
	N3Percent           *string `json:"n_3_percent"`
	CarbohydratePercent *string `json:"carbohydrate_percent"`
	ProteinPercent      *string `json:"protein_percent"`
}

// DRIRequirements is the outer object block containing the split macro/micro structures.
type DRIRequirements struct {
	Elements       DRIElements       `json:"elements"`
	Vitamins       DRIVitamins       `json:"vitamins"`
	Macronutrients DRIMacronutrients `json:"macronutrients"`
}

// DRI dictates the root schema persisted as a row corresponding to one specific life stage.
type DRI struct {
	ID             uint            `json:"id"               gorm:"primaryKey;autoIncrement"`
	LifeStageGroup string          `json:"life_stage_group" gorm:"index"`
	AgeRange       string          `json:"age_range"        gorm:"index"`
	RdaAi          DRIRequirements `json:"rda_ai"           gorm:"type:jsonb;serializer:json"`
	Ear            DRIRequirements `json:"ear"              gorm:"type:jsonb;serializer:json"`
	Ul             DRIRequirements `json:"ul"               gorm:"type:jsonb;serializer:json"`
	Amdr           DRIAmdr         `json:"amdr"             gorm:"type:jsonb;serializer:json"`
}

// DRIRepository defines the data access boundary for DRI lookups.
type DRIRepository interface {
	GetByDemographic(ctx context.Context, lifeStage string, ageRange string) (*DRI, error)
}
