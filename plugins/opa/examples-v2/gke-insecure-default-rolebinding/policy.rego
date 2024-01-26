package fairwinds

defaultRoleSubjects(subject) {
	defaultSubjects := {"system:anonymous", "system:unauthenticated", "system:authenticated"}
	defaultSubjects[subject]
}

checkRoleBinding[actionItem] {
	input.kind == "RoleBinding"
	subject := input.subjects[_]

	defaultRoleSubjects(subject.name)

	actionItem := {
		"title": "Insecure GKE RoleBinding",
		"description": sprintf("RoleBinding %s with subject %s is insecure", [input.metadata.name, subject.name]),
		"remediation": "There should be no RoleBinding that references `system:authenticated`, `system:unauthenticated`, and `system:anonymous` groups.",
		"category": "Security",
		"severity": .99,
	}
}
