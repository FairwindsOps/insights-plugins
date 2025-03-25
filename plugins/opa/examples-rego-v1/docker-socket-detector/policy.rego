package fairwinds

import rego.v1

blockedNamespace(elem) if {
	# List Kubernetes namespaces where this policy should not be applied.
	blockedNamespaces := [""]

	ns := blockedNamespaces[_]
	elem.metadata.namespace == ns
}

podHasDockerSocketVolume(pod) if {
	# Get the volumes from a pod-spec
	volume := pod.spec.volumes[_]
	path := volume.hostPath.path
	contains(path, "docker.sock")
}

checkCronjob contains actionItem if {
	not blockedNamespace(input)
	input.kind == "CronJob"
	pod := input.spec.jobTemplate.spec.template
	podHasDockerSocketVolume(pod)
	actionItem := {
		"title": concat(" ", [input.kind, "has a volume using the docker-socket"]),
		"description": "Docker-socket mounts are being removed in a future release of kubernetes",
		"remediation": "Please do not mount the docker-socket",
		"category": "Reliability",
		"severity": .6,
	}
}

checkDeploymentLike contains actionItem if {
	not blockedNamespace(input)
	kinds := {"Deployment", "DaemonSet", "StatefulSet", "ReplicaSet", "Job"}
	kind := kinds[_]
	input.kind == kind
	pod := input.spec.template
	podHasDockerSocketVolume(pod)
	actionItem := {
		"title": concat(" ", [input.kind, "has a volume using the docker-socket"]),
		"description": "Docker-socket mounts are being removed in a future release of kubernetes",
		"remediation": "Please do not mount the docker-socket",
		"category": "Reliability",
		"severity": .6,
	}
}

checkPod contains actionItem if {
	not blockedNamespace(input)
	input.kind == "Pod"

	# Only alert for stand-alone pods,
	# avoiding duplicate action-items for pods which belong to a controller.
	not input.metadata.ownerReferences
	podHasDockerSocketVolume(input)
	actionItem := {
		"title": concat(" ", [input.kind, "has a volume using the docker-socket"]),
		"description": "Docker-socket mounts are being removed in a future release of kubernetes",
		"remediation": "Please do not mount the docker-socket",
		"category": "Reliability",
		"severity": .6,
	}
}
