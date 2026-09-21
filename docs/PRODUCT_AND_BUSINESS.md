# NutriX — Product & Business Documentation

> **Document Type**: Product & Business Analysis
> **Version**: 1.0
> **Last Updated**: 2026-08-25
> **Evidence**: Full codebase analysis, `ARCHITECTURE.md`, `internal/nutrition/usecase/nutrition_usecase.go`

---

## Problem Statement

### The Three Core Problems

#### 1. Food Tracking is Broken [IMPLEMENTED — Technical Solution]

**Pain Point**: Manual calorie and macro logging requires users to:
- Look up every food item in a database
- Estimate portion sizes
- Calculate nutritional values
- Do this 3-5 times per day, every day

**Reality**: 80% of nutrition apps are abandoned within the first month. The primary reason is tracking burden.

**Current Solution**: NutriX provides:
- Fast food search with local database (VFA + USDA)
- Spoonacular API fallback for missing foods
- Computer vision to auto-recognize foods from photos (Phase 5)
- One-tap logging once food is identified

#### 2. Health Conditions Make Food Choices Complex [PARTIAL — Technical Solution]

**Pain Point**: Users with medical conditions face daily decisions:
- Diabetics need to track carbs and glycemic impact
- Kidney disease patients must limit sodium, potassium, phosphorus
- Gout sufferers avoid high-purine foods
- Heart disease patients watch sodium and saturated fat

**Current Gap**: Generic nutrition apps show "calories" but can't answer: "Is this safe for me given my condition?"

**NutriX Solution**: [PARTIAL]
- Knowledge Graph (ai-kg) stores disease-food interaction rules
- Safety analysis via `AnalyzeFood` and `AnalyzeMeal` gRPC calls
- Threshold snapshots for disease-specific limits
- **Gap**: Disease context not fully wired to all KG calls

#### 3. One-Size-Fits-All Advice Doesn't Work [PARTIAL — Technical Solution]

**Pain Point**: Users want personalized guidance that accounts for:
- Their specific biometrics (height, weight, age, gender)
- Activity level and workout habits
- Cultural food preferences (Vietnamese cuisine)
- Dietary restrictions (vegan, halal)
- Personal dislikes and exclusions

**NutriX Solution**: [IMPLEMENTED]
- DRI (Dietary Reference Intake) based personalized targets
- BMR/TDEE calculation from user biometrics
- Vietnamese food database (VFA) as first-class citizen
- User portfolio with preferred/disliked/excluded ingredients

---

## User Pain Points

### Pain Point Matrix

| Pain Point | Severity | Current Mitigation | Remaining Gap |
|---|---|---|---|
| Manual logging is tedious | 🔴 High | Food search + CV (partial) | Full CV accuracy not validated |
| Don't know if food is safe | 🔴 High | KG safety analysis | Context not fully wired |
| Generic advice doesn't fit | 🟡 Medium | DRI targets + profile | KG reasoning limited |
| Forgot to log meals | 🟡 Medium | Daily reminders (client) | Not backend-controlled |
| Portion estimation hard | 🟡 Medium | Reference weight support | No visual guidance |
| Vietnamese foods missing | 🟡 Medium | VFA database | Limited to 30 dishes |
| Need workout context | 🟢 Low | MET-based burn tracking | External API dependency |

---

## Existing Workflow

### Current User Journey (Without NutriX)

```
1. Open generic nutrition app
2. Search "pho" → generic entry found
3. Manually adjust portion to "1 bowl"
4. Guess calories from displayed range
5. Repeat for 3-5 meals per day
6. No understanding of sodium/purine for gout condition
7. No personalized recommendations
8. App abandoned after 2 weeks
```

### Improved Workflow (With NutriX)

```
1. Open NutriX app
2. Take photo of meal
3. CV recognizes "Phở Bò" → resolves to database entry
4. AI estimates portion weight from photo
5. User confirms or adjusts
6. System checks KG for gout condition → safe ✓
7. Daily plan updates with new entry
8. Weekly analytics show progress toward goals
```

---

## Value Proposition

### For End Users

| Value | Description | Evidence |
|---|---|---|
| **Time Savings** | Reduce logging from 5 min to 30 sec per meal | CV photo → food resolution |
| **Health Safety** | Know if food is risky for your conditions | KG AnalyzeFood + AnalyzeMeal |
| **Personalization** | Advice that fits your body, culture, preferences | User profiles + VFA database |
| **Motivation** | Streaks and achievements keep engagement | Gamification system |
| **Insights** | Weekly/monthly trends without manual calculation | Analytics aggregation |

### For Business

| Value | Description | Evidence |
|---|---|---|
| **Data Moat** | Vietnamese food database | 30+ VFA dishes + ingredients |
| **AI Differentiation** | Knowledge Graph for health conditions | Neo4j + disease rules |
| **Scalability** | Cloud-native, containerized | Docker Compose production stack |
| **API-First** | Easy integrations and partner channels | REST API + gRPC |

---

## Business Benefits

### Direct Revenue Opportunities

1. **Premium Subscriptions** [POTENTIAL]
   - Advanced AI analysis features
   - Custom meal plan generation
   - Priority support

2. **B2B API Access** [POTENTIAL]
   - Healthcare provider integrations
   - Corporate wellness programs
   - Nutritionist practice tools

3. **Data Insights** [FUTURE]
   - Anonymized population nutrition trends
   - Supplement brand partnerships

### Indirect Benefits

- User acquisition through superior UX
- Retention through gamification
- Word-of-mouth through Vietnamese community

---

## Differentiation

### Actual Differentiators [IMPLEMENTED]

| Differentiator | Description | Evidence |
|---|---|---|
| **Vietnamese Food Focus** | First-class VFA database for VN cuisine | `vfa_food_db.json`, `vfa_dishes_db.json` |
| **Health Condition AI** | Knowledge Graph for disease-food safety | `ai-kg` service + Neo4j |
| **End-to-End Pipeline** | Photo → CV → KG → Recommendation | `AnalyzeMealImage` gRPC endpoint |
| **Privacy-Preserving** | Blind-indexed encryption for allergies | `AllergiesBidx`, `MedicalConditionsBidx` |

### Potential Differentiators [ARCHITECTURE ENABLES]

| Differentiator | Current State | Path to Implementation |
|---|---|---|
| Real-time safety alerts | Concept only | Add webhook notifications |
| Nutrition coach chatbot | Proto exists | Expand KG reasoning + LLM |
| Meal prep planning | Experimental | Scale in-memory cache to Redis |
| Supplement recommendations | Not designed | Add to KG + recommendation engine |

### Weak Differentiators

| Claim | Why Weak |
|---|---|
| "AI-powered nutrition" | Generic claim; competitors claim same |
| "Personalized plans" | Limited by incomplete context wiring |
| "Tracks everything" | Only 4 macros; micronutrients in unstructured JSONB |

---

## Current Market/Product Scope

### Geographic Focus

- **Primary**: Vietnam (VFA food database, Vietnamese localization)
- **Secondary**: English-speaking markets (USDA fallback)

### User Segment Focus

1. **Health-conscious millennials** — primary target
2. **Vietnamese diaspora** — differentiated by VFA database
3. **Fitness enthusiasts** — workout integration
4. **Users with chronic conditions** — safety features (limited by context wiring)

### Pricing Model

- **Free tier**: Basic logging + analytics
- **Premium tier**: AI features (not yet monetized)
- **B2B tier**: API access (not yet available)

---

## Potential Use Cases

### Consumer Use Cases [IMPLEMENTED/PRIMARY]

1. **Daily meal logging** — track calories, macros, water
2. **Weight management** — deficit/surplus tracking with TDEE
3. **Gym nutrition** — protein intake for muscle building
4. **Medical compliance** — food safety for conditions (limited)

### Enterprise Use Cases [POTENTIAL]

1. **Corporate wellness** — employee nutrition programs
2. **Healthcare integration** — dietitian platforms
3. **Insurance wellness** — rewards for healthy eating

---

## Known Commercial Limitations

| Limitation | Impact | Mitigation |
|---|---|---|
| No payment integration | Can't monetize premium features | Add Stripe/VNPay |
| No multi-tenant SaaS | Can't serve B2B customers | Add org/team model |
| No white-label option | Can't license to partners | Extract to separate repo |
| Spoonacular rate limits | External dependency risk | Build own food DB |

---

## Current Value vs Potential Value

### Current Value [IMPLEMENTED]

| Feature | Value Delivered |
|---|---|
| Food search + logging | High — core use case works |
| Analytics + streaks | High — gamification drives retention |
| Workout integration | Medium — adds completeness |
| Vietnamese database | High — unique for target market |
| Basic personalization | Medium — DRI targets work |

### Potential Value [PLANNED]

| Feature | Value Potential |
|---|---|
| Full KG context wiring | High — enables health condition use case |
| Real-time safety alerts | High — premium feature |
| AI meal planning | Medium — adds convenience |
| Nutrition coaching | High — premium tier |

### Future Opportunity

1. **Healthcare Compliance**: HIPAA/GDPR compliance for medical data
2. **Nutritionist Portal**: Dashboard for dietitians to monitor clients
3. **Meal Kit Integration**: Partner with meal kit services
4. **Wearable Integration**: Apple Health, Google Fit sync

---

## Risks

| Risk | Likelihood | Impact | Mitigation |
|---|---|---|---|
| Spoonacular API changes | Medium | Medium | Build internal food DB |
| KG accuracy issues | Medium | High | Add user feedback loop |
| Data privacy regulations | Low | High | Blind indexing already in place |
| AI hallucination | Medium | High | Keep human-in-the-loop |
| Technical debt | High | Medium | Refactor incomplete features |
