package example.terraform.s3

pi := 3.14

default allow = false

default deny = true

allow = true {
	count(violation) == 0
}

deny = true {
    count(violation) > 0
}

violation[resource.address] {
	some resource
    public_bucket[resource]
}

buckets[r] {
    r := input.resource_changes[_]     # exists in resource_changes
    r.type = "aws_s3_bucket"           # type of resource is "aws_s3_bucket"
}

public_bucket[b] {
	b := buckets[_]                    # change is a bucket
    b.change.actions[_] == data.example.terraform.update_actions[_]  # created or updated
    b.change.after.acl == "public"     # acl after change is 'public'
}
