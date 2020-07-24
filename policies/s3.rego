package terraform.s3

default pass = false

allow = true {
	count(violations) == 0
}

violations[{"addr": r.address, "msg": msg}] {
	some r
    public_bucket_acls[r]
    msg := "must not have public ACL"
}

buckets[r] {
    r := input.resource_changes[_]     # exists in resource_changes
    r.type = "aws_s3_bucket"           # type of resource is "aws_s3_bucket"
}

public_bucket_acls[b] {
	b := buckets[_]                    # change is a bucket
    b.change.actions[_] == data.terraform.update_actions[_]  # created or updated
    b.change.after.acl == "public"     # acl after change is 'public'
}
