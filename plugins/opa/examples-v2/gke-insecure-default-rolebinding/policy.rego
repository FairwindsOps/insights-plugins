package fairwinds

checkRoleBinding[actionItem] {
	input.kind == "RoleBinding"
	subject := input.subjects[_]

	defaultSubjects := {"system:anonymous", "system:unauthenticated", "system:authenticated"}
	defaultSubjects[_] = subject.name

	actionItem := {
		"title": "Insecure GKE RoleBinding",
		"description": sprintf("RoleBinding %s contains default system subject", [input.metadata.name]),
		"remediation": "There should be no RoleBinding that references `system:authenticated`, `system:unauthenticated`, and `system:anonymous` groups.",
		"category": "Security",
		"severity": .99,
	}
}
