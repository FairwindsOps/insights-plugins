package fairwinds

defaultClusterRoleSubjects(subject) {
	defaultSubjects := {"system:anonymous", "system:unauthenticated", "system:authenticated"}
	defaultSubjects[subject]
}

isAllowedClusterRoleBindingRoleRef(roleRefName) {
	allowedRoleRefNames := {"system:basic-user", "system:discovery", "system:public-info-viewer"}
	allowedRoleRefNames[roleRefName]
}

checkClusterRoleBinding[actionItem] {
	input.kind == "ClusterRoleBinding"
	subject := input.subjects[_]
	roleRefName := input.roleRef.name
	defaultClusterRoleSubjects(subject.name)
	not isAllowedClusterRoleBindingRoleRef(roleRefName)

	actionItem := {
		"title": "Insecure GKE ClusterRoleBinding",
		"description": sprintf("ClusterRoleBinding %s with subject %s contains non-default binding to role %s", [input.metadata.name, subject.name, roleRefName]),
		"remediation": "Only the expected default bindings should exist to the `system:authenticated`, `system:unauthenticated`, and `system:anonymous` groups.",
		"category": "Security",
		"severity": .99,
	}
}
