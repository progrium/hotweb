import * as hero from "/lib/hero.mjs";
import * as cta from "/lib/cta.mjs";
import * as footer from "/lib/footer.mjs";
import * as nav from "/lib/nav.mjs";
import * as features from "/lib/features.mjs";

export const Page = {
    view: function (vnode) {
        return m("main", [
            m(hero.HeroSection, [
                m(hero.HeroHead, m(nav.NavBar)),
                m(hero.HeroBody, m("div.container.has-text-centered",
                    [
                        m("h1.title",
                            [
                                " The new standard in ",
                                m("span[id='new-standard']",
                                    "hot reloading"
                                )
                            ]
                        ),
                        m("h2.subtitle",
                            " Lorem ipsum dolor sit amet, consectetur adipiscing elit, sed do eiusmod tempor incididunt ut labore et dolore magna aliqua. Ut enim ad minim veniam, quis nostrud exercitation ullamco laboris nisi ut aliquip ex ea commodo consequat. "
                        )
                    ]
                ))
            ]),
            m(cta.CTA),
            m(features.Features),
            m(footer.Footer),
        ]);
    }
}