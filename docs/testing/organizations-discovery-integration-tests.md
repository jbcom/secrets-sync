# AWS Organizations Dynamic Discovery - Integration Tests

This document describes how to run integration tests for AWS Organizations dynamic discovery.

## Prerequisites

### AWS Permissions

Your AWS execution context needs the following IAM permissions:

```json
{
  "Version": "2012-10-17",
  "Statement": [
    {
      "Effect": "Allow",
      "Action": [
        "organizations:DescribeOrganization",
        "organizations:ListAccounts",
        "organizations:ListAccountsForParent",
        "organizations:ListOrganizationalUnitsForParent",
        "organizations:ListTagsForResource"
      ],
      "Resource": "*"
    }
  ]
}
```

### Test Account Setup

For comprehensive testing, set up test accounts with the following tags:

```bash
# Tag test accounts for filtering
aws organizations tag-resource \
  --resource-id 123456789012 \
  --tags Key=Environment,Value=production Key=Team,Value=platform

aws organizations tag-resource \
  --resource-id 234567890123 \
  --tags Key=Environment,Value=development Key=Team,Value=platform

aws organizations tag-resource \
  --resource-id 345678901234 \
  --tags Key=Environment,Value=staging Key=Team,Value=analytics
```

## Integration Test Scenarios

### Test 1: OU-Based Discovery (Non-Recursive)

**Configuration:**
```yaml
dynamic_targets:
  test_ou_discovery:
    discovery:
      organizations:
        ou: "ou-xxxx-development"
    imports:
      - test-secrets
```

**Expected Behavior:**
- Lists only accounts directly in the specified OU
- Does not traverse child OUs
- Accounts have ID, Name, Email, and Status populated
- No tag filtering applied

**Validation:**
```bash
# Run the pipeline in dry-run mode
./vss pipeline --config test-config.yaml --dry-run

# Verify discovered targets in output
# Should show accounts directly in the OU only
```

### Test 2: Recursive OU Traversal

**Configuration:**
```yaml
dynamic_targets:
  test_recursive_discovery:
    discovery:
      organizations:
        ou: "ou-xxxx-workloads"
        recursive: true
    imports:
      - test-secrets
```

**Expected Behavior:**
- Lists accounts in the specified OU
- Recursively traverses all child OUs
- Includes accounts from all nested OUs
- No tag filtering applied

**Validation:**
```bash
# Count accounts in the OU hierarchy manually
aws organizations list-accounts-for-parent --parent-id ou-xxxx-workloads
aws organizations list-organizational-units-for-parent --parent-id ou-xxxx-workloads
# Then check each child OU

# Compare with discovery output
./vss pipeline --config test-config.yaml --dry-run | grep "Discovered target"
```

### Test 3: Tag-Based Filtering (All Accounts)

**Configuration:**
```yaml
dynamic_targets:
  test_tag_filtering:
    discovery:
      organizations:
        tags:
          Environment: production
          Team: platform
    imports:
      - test-secrets
```

**Expected Behavior:**
- Lists ALL accounts in the organization
- Filters to only accounts with BOTH tags matching
- Accounts without tags are excluded
- Both tag key and value must match exactly (case-sensitive)

**Validation:**
```bash
# Manually verify which accounts have matching tags
for account in $(aws organizations list-accounts --query 'Accounts[].Id' --output text); do
  echo "Account: $account"
  aws organizations list-tags-for-resource --resource-id $account
done

# Compare with discovery output
./vss pipeline --config test-config.yaml --dry-run
```

### Test 4: Combined OU and Tag Filtering

**Configuration:**
```yaml
dynamic_targets:
  test_combined_filtering:
    discovery:
      organizations:
        ou: "ou-xxxx-production"
        tags:
          Environment: production
    imports:
      - test-secrets
```

**Expected Behavior:**
- Lists accounts in the specified OU only
- Filters to only accounts with matching tags
- Does not search other OUs
- Both OU membership and tags must match

**Validation:**
```bash
# Verify accounts in OU
aws organizations list-accounts-for-parent --parent-id ou-xxxx-production

# Check tags on those accounts
for account in $(aws organizations list-accounts-for-parent --parent-id ou-xxxx-production --query 'Accounts[].Id' --output text); do
  echo "Account: $account"
  aws organizations list-tags-for-resource --resource-id $account | grep Environment
done

# Run discovery
./vss pipeline --config test-config.yaml --dry-run
```

### Test 5: Recursive OU with Tag Filtering

**Configuration:**
```yaml
dynamic_targets:
  test_recursive_with_tags:
    discovery:
      organizations:
        ou: "ou-xxxx-workloads"
        recursive: true
        tags:
          AutoManaged: enabled
    imports:
      - test-secrets
```

**Expected Behavior:**
- Recursively lists accounts from OU and all child OUs
- Filters results by tags
- Combination of recursive traversal and tag filtering
- Most comprehensive discovery method

**Validation:**
```bash
# This requires checking all accounts in the OU hierarchy
# and verifying they have the required tags
./vss pipeline --config test-config.yaml --dry-run --log-level debug
```

### Test 6: Account Name Resolution

**Configuration:**
```yaml
dynamic_targets:
  test_name_resolution:
    discovery:
      identity_center:
        group: "TestGroup"
    imports:
      - test-secrets
```

**Expected Behavior:**
- Discovers accounts from Identity Center
- Automatically enriches with account names from Organizations API
- Generated target names use account names when available
- Falls back to account ID if name not available

**Validation:**
```bash
# Check that discovered targets have meaningful names
./vss pipeline --config test-config.yaml --dry-run | grep "Discovered target"

# Names should be sanitized:
# "Analytics Sandbox" → "Analytics_Sandbox"
# "Test (Dev)" → "Test_Dev"
```

## Manual Testing Checklist

- [ ] OU-based discovery (non-recursive) works
- [ ] Recursive OU traversal discovers all nested accounts
- [ ] Tag filtering correctly filters accounts
- [ ] Combined OU + tag filtering works
- [ ] Account names are resolved from Organizations
- [ ] Exclusion list correctly excludes accounts
- [ ] Multiple discovery methods combine results
- [ ] Permission errors are handled gracefully
- [ ] Invalid OU IDs produce helpful errors
- [ ] Performance is acceptable for large organizations
- [ ] Target names are sanitized correctly
- [ ] Duplicate accounts are deduplicated
- [ ] Tag keys and values are case-sensitive
- [ ] Accounts without tags are excluded when filtering by tags

## Debugging Tips

### Enable Debug Logging

```bash
./vss pipeline --config config.yaml --dry-run --log-level debug
```

### Common Issues

1. **"no access to Organizations API"**
   - Check execution context has Organizations permissions
   - Verify running from management account or delegated admin

2. **Tags not being applied**
   - Verify tags exist on accounts using AWS CLI
   - Check tag keys and values match exactly (case-sensitive)
   - Ensure accounts have the Tags field populated

3. **Recursive traversal not working**
   - Check ListOrganizationalUnitsForParent permission
   - Verify OU ID is correct
   - Check debug logs for OU traversal errors
