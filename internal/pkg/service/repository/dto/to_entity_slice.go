package dto

// ToEntitySlice мапинг []*dto.Record -> []*entity.Entity при условии наличия у *dto.Record метода Entity()
// Могут быть непонятки у компилятора.
func ToEntitySlice[
	EntitySlice ~[]Entity,
	DTOSlice ~[]DTO,
	Entity any,
	DTO interface{ Entity() Entity },
](dto DTOSlice,
) EntitySlice {
	if dto == nil {
		return nil
	}

	entities := make(EntitySlice, 0, len(dto))

	for _, record := range dto {
		entities = append(entities, record.Entity())
	}

	return entities
}
