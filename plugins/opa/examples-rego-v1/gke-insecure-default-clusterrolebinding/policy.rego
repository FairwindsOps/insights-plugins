package fairwinds

import rego.v1

isAllowedClusterRoleBindingRoleRef(roleRefName) if {
	allowedRoleRefNames := {"system:basic-user", "system:discovery", "system:public-info-viewer"}
	allowedRoleRefNames[roleRefName]
}

checkClusterRoleBinding contains actionItem if {
	input.kind == "ClusterRoleBinding"
	defaultSubjects := {"system:anonymous", "system:unauthenticated", "system:authenticated"}
	inputSubjectsSet := {x | x = input.subjects[_]}
	count(defaultSubjects - inputSubjectsSet) == count(defaultSubjects)
	roleRefName := input.roleRef.name
	not isAllowedClusterRoleBindingRoleRef(roleRefName)

	actionItem := {
		"title": "Insecure GKE ClusterRoleBinding",
		"description": sprintf("ClusterRoleBinding %s references a default role", [input.metadata.name]),
		"remediation": "Only the expected default bindings should exist to the `system:authenticated`, `system:unauthenticated`, and `system:anonymous` groups.",
		"category": "Security",
		"severity": .99,
	}
}
