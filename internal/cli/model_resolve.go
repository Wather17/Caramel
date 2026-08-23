package cli

// resolveModel define o modelo efetivo com prioridade:
//  1. flag explicitamente passada pelo usuário (ex: -m anthropic/...)
//  2. valor configurado no .env (ex: MODEL_IMAGE)
//  3. default de fábrica (flagVal sem uso explícito já carrega a constante)
func resolveModel(flagVal string, flagChanged bool, cfgVal string) string {
	if flagChanged && flagVal != "" {
		return flagVal
	}
	if cfgVal != "" {
		return cfgVal
	}
	return flagVal
}