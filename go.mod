module github.com/g-uva/kube-energy-scheduler

go 1.20

require (
    k8s.io/api v0.28.8
    k8s.io/apimachinery v0.28.8

    // this is the core monorepo for scheduler/framework, etc.
    k8s.io/kubernetes v1.28.8

    // the out-of-tree config API for the scheduler
    k8s.io/kube-scheduler/config v0.28.8
)

// whenever you require k8s.io/kubernetes, actually pull from the GitHub repo
replace k8s.io/kubernetes => github.com/kubernetes/kubernetes v1.28.8
