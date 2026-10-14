Here are the steps for setting up the workflow:

1. Set up the PocketBase server in `./api` to store the questionnaire and step data.
2. Verify BankID functionality using `docs/bankid-testing.md` as the main reference.
3. The user should provide context files inside `./context` to establish which questions to create for the questionnaire and the mobile app's specific purpose.
4. Populate the PocketBase database with questions gathered from the context file, use `./docs/questionnaire-data.md` for additional context on how to structure the questions. Populate the data using a Go migration to seed the specific questions
5. Adapt the source using [the customization guide](customization.md). Shared widgets live in `lib/ui/`; questionnaire inputs live in `lib/questionnaires/widgets/`. Study copy is in `lib/study/`, flow composition in `lib/app/`, and collection behavior in `lib/steps/`.
6. Install the Flutter dependencies and build the project. The app's main target platform is iOS.
7. Deploy the project using the contents inside `./deploy`.
8. Verify the deployed application via health checks.
9. Run the automated suites and [manual review](manual-review.md), including physical-device BankID return and health permissions.
