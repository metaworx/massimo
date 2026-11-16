# NOTE StoreRaise v1.2: decisions on block 4

The owner, 2026-11-16, `EXEC D18-A D19-A`, on the gate after Block 4
(NOTE StoreRaise v1.2, "how block 4 went", points 1 and 2):

- **D18-A:** a user's `integration` in the API is an object: `uuid`, and
  `id`, `name`, `linked_to` for one of its users, as AP IntegrationUsers'
  mark has it. As #339 does it.
- **D19-A:** a log or audit row's `effective_user` and `real_user` in the
  API are objects (`name`, `display_name`, `integration`, `display`). The
  tables keep `effective_user_id` and `real_user_id`, ids into `users`;
  only the JSON carries the objects. As #339 does it.
