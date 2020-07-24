package terraform

# Actions that create new resources or update existing ones
update_actions := ["create", "modify"]

# Actions that destroy things
destroy_actions := ["delete"]

default pass = false

pass = true {
    count(violations) == 0
}

test := {{"msg": "foo", "addr": "foo.bar"}}

violations = v {
    v := 
        data.terraform.tagging.violations | 
        data.terraform.s3.violations
}
