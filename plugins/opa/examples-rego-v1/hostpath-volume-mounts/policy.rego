package fairwinds

import rego.v1

blockedNamespace(elem) if {
	# List Kubernetes namespaces where this policy should not be applied.
	blockedNamespaces := []

	ns := blockedNamespaces[_]
	elem.metadata.namespace == ns
}

podHasHostPathVolume(pod) if {
	# Get the volumes from each container
	volumes := pod.spec.volumes[_]
	volumes.hostPath
}

checkCronjob contains actionItem if {
	not blockedNamespace(input)
	input.kind == "CronJob"
	pod := input.spec.jobTemplate.spec.template
	podHasHostPathVolume(pod)
	actionItem := {
		"title": concat(" ", [input.kind, "has a volume using hostPath"]),
		"description": "hostPath mounts are generally considered insecure",
		"remediation": "Please do not use hostPath volumes",
		"category": "Security",
		"severity": .9,
	}
}

checkDeploymentLike contains actionItem if {
	not blockedNamespace(input)
	kinds := {"Deployment", "DaemonSet", "StatefulSet", "ReplicaSet", "Job"}
	kind := kinds[_]
	input.kind == kind
	pod := input.spec.template
	podHasHostPathVolume(pod)
	actionItem := {
		"title": concat(" ", [input.kind, "has a volume using hostPath"]),
		"description": "hostPath mounts are generally considered insecure",
		"remediation": "Please do not use hostPath volumes",
		"category": "Security",
		"severity": .9,
	}
}

checkPod contains actionItem if {
	not blockedNamespace(input)
	input.kind == "Pod"

	# Only alert for stand-alone pods,
	# avoiding duplicate action-items for pods which belong to a controller.
	not input.metadata.ownerReferences
	podHasHostPathVolume(input)
	actionItem := {
		"title": concat(" ", [input.kind, "has a volume using hostPath"]),
		"description": "hostPath mounts are generally considered insecure",
		"remediation": "Please do not use hostPath volumes",
		"category": "Security",
		"severity": .9,
	}
}
