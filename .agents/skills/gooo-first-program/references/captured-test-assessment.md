# Captured test assessment

Use this when the user wants to inspect how Gooo compares a declared test plan
with saved test outcomes. Follow the
[source-pinned example guide](https://github.com/kimjooyoon/gooo-go-ports/blob/8d91f0f28bf9d605e072aa0326a5d6d7cb1e4ed2/examples/assessment-session/README.md).

That example uses a separate, explicitly selected development compiler at
`276734167edc7f5fef18cbb79004ea9592553317` and Go 1.27.3. Its Go host prints
`program_value` and `assessment_execution` separately and retains full records
in a new persistent directory. Keep the existing tutorial compiler available.

Read the four sample results using both objects: a domain mismatch can be the
accurately expected result, and the deliberately wrong expected state remains
`PROGRESS`, 0/1. The guide specifies the historical fixtures, finite counts,
source/request/capture bindings and reproduction commands. Run its offline
record checks after the actual session and report the observed values and
record paths.
