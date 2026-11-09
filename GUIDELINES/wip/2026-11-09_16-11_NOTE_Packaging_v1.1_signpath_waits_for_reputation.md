# NOTE Packaging v1.1: signpath waits for reputation

A decision of the owner's on Block 4 of
[AP Packaging v1.1](2026-11-06_13-55_AP_Packaging_deb_apt_homebrew.md),
taken on 2026-11-09 after reading SignPath Foundation's application form.
The plan's D5, no Windows code signing for now, stands.

1. **What the form asks**, read at https://signpath.org/apply on 2026-11-09
   (the analysis of 2026-11-06, section 4.1, had the terms, not the form):
   - **Project Name**, required:
     "A Google search for this name should clearly identify your project."
     "Massimo" alone finds a clothing brand and a vehicle maker;
     a search for it with "MAX! Cube" and "metaworx" found nothing that day.
   - **Reputation**, required: links or information showing that the project
     is widely used or trusted, such as media coverage, blog posts,
     download statistics, GitHub insights or community discussions.
   - **Download URL**, optional: the page "must mention that the project uses
     the SignPath Foundation for code signing."
   - Also required: the repository and homepage addresses, a tagline,
     a category, a description, the build system,
     and the applicant's name and e-mail address for a SignPath account;
     the form sits behind a reCAPTCHA.
2. **Where Massimo stood** on 2026-11-09:
   the repository created on 2026-11-03, without stars, forks or watchers;
   14 releases with 2,555 asset downloads, many of them the owner's own boxes;
   no third-party mention found;
   the site, `massimo.metaworx.ch`, live only with the next release.
   The owner judged that an application would be refused,
   and the read of the form agreed.
3. **D23-A, the owner's:**
   Windows stays unsigned (D5),
   and the application to SignPath waits until the Reputation field has something to name:
   - the site live, with its download and privacy pages;
   - a thread or two where MAX! users are:
     the FHEM, LoxBerry, ioBroker and openHAB communities;
   - some stars and links from outside.

   The analysis's fallback holds from the application on:
   declined, or no answer two months after it, then Certum in the cloud.
   The name given in the form should be one a search finds,
   such as "Massimo, a server for eQ-3 MAX! Cubes".
4. **The two alternatives weighed and left:**
   - **D23-B, Certum's Open Source Code Signing in the Cloud:**
     no reputation test, identity only; 49 EUR a year;
     the owner's name on every dialog;
     the reputation restarting with each yearly certificate;
     "Product is out of stock" again on 2026-11-09.
   - **D23-C, Azure Artifact Signing:**
     9.99 USD a month;
     Microsoft's newer quickstart lists Switzerland for organisations
     and individuals only in the US and Canada,
     so it needs metaworx to qualify as an organisation,
     with three years of verifiable history in the analysis's reading.
5. **Unsigned meanwhile** costs what `README.md` already says under Windows:
   SmartScreen's "Windows protected your PC" on a double-click in Explorer,
   passed with "Run anyway", and nothing from a console;
   `massimo update` never meets SmartScreen (`docs/update-windows.md`,
   "Defender and SmartScreen").

Sources, read on 2026-11-09:
https://signpath.org/apply;
https://learn.microsoft.com/azure/artifact-signing/quickstart;
https://shop.certum.eu/open-source-code-signing-on-simplysign.html.
