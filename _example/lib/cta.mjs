export const CTA = {
    view: function (vnode) {
        return m("div.box.cta",
            m("p.has-text-centered",
                [
                    m("span.tag.is-primary",
                        "New"
                    ),
                    " Ut enim ad minim veniam, quis nostrud exercitation ullamco laboris nisi ut aliquip ex ea commodo consequat. "
                ]
            )
        )
    }
}