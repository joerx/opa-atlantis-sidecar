package terraform.tagging

default pass = false

pass = true {
    count(violations) == 0
}

violations[{"addr": addr, "msg": msg}] {
    some addr
    tags := missing_tags[addr]
    count(tags) > 0
    msg := sprintf("missing tags '%s'", [concat("', '", tags)])
}

required_tag := {"Service", "Owner", "Environment"}

taggable := [
    "aws_s3_bucket",
    "aws_instance",
    "aws_rds_instance"
]

change_to_taggable[r] {
    r := input.resource_changes[_]
    r.change.actions[_] == data.terraform.update_actions[_]
    r.type == taggable[_]
}

missing_tags[r.address] = tags {
    r := change_to_taggable[_]
    tags := [tag | some tag
                    required_tag[tag]                       # for any required tag
                    resource_tags := r.change.after.tags    # where list of tags on a resource after change
                    not resource_tags[tag]]                 # does not contain a tag with the required
}
