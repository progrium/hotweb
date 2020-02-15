export const Footer = {
    view: function (vnode) {
        return m("footer.footer", 
            m("div.container",
                [
                    m("div.content.has-text-centered",
                        [
                            m("div.control.level-item",
                                m("a[href='https://github.com/BulmaTemplates/bulma-templates']",
                                    m("div.tags.has-addons",
                                        [
                                            m("span.tag.is-dark",
                                                "Bulma Templates"
                                            ),
                                            m("span.tag.is-info",
                                                "MIT license"
                                            )
                                        ]
                                    )
                                )
                            )
                        ]
                    )
                ]
            )
        )
    }
}