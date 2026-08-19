/*
Copyright (C) 2023-2026 QuantumNous

This program is free software: you can redistribute it and/or modify
it under the terms of the GNU Affero General Public License as
published by the Free Software Foundation, either version 3 of the
License, or (at your option) any later version.

This program is distributed in the hope that it will be useful,
but WITHOUT ANY WARRANTY; without even the implied warranty of
MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE. See the
GNU Affero General Public License for more details.

You should have received a copy of the GNU Affero General Public License
along with this program. If not, see <https://www.gnu.org/licenses/>.

For commercial licensing, please contact support@quantumnous.com
*/
import assert from 'node:assert/strict'
import { describe, test } from 'node:test'

import * as React from 'react'
import { renderToStaticMarkup } from 'react-dom/server'

import { ChannelModelBadges } from './channel-model-badges'

const testGlobal = globalThis as typeof globalThis & { React: typeof React }
testGlobal.React = React

describe('ChannelModelBadges', () => {
  test('renders a dash for an empty model list', () => {
    const markup = renderToStaticMarkup(
      React.createElement(ChannelModelBadges, { models: '' })
    )
    assert.match(markup, />-</)
  })

  test('renders one or two models without an overflow count', () => {
    const one = renderToStaticMarkup(
      React.createElement(ChannelModelBadges, { models: 'model-a' })
    )
    const two = renderToStaticMarkup(
      React.createElement(ChannelModelBadges, { models: 'model-a,model-b' })
    )

    assert.match(one, /model-a/)
    assert.doesNotMatch(one, /\+1/)
    assert.match(two, /model-a/)
    assert.match(two, /model-b/)
    assert.doesNotMatch(two, /\+1/)
  })

  test('shows two models, an overflow count, and all models in the tooltip', () => {
    const element = ChannelModelBadges({
      models: 'model-a,model-b,model-c',
    })
    const props = element.props as {
      max: number
      items: React.ReactNode[]
    }
    const markup = renderToStaticMarkup(
      React.createElement(ChannelModelBadges, {
        models: 'model-a,model-b,model-c',
      })
    )
    const tooltipMarkup = renderToStaticMarkup(
      React.createElement(React.Fragment, null, ...props.items)
    )

    assert.equal(props.max, 2)
    assert.equal(props.items.length, 3)
    assert.match(markup, /model-a/)
    assert.match(markup, /model-b/)
    assert.match(markup, /\+1/)
    assert.match(markup, /tabindex="0"/)
    assert.match(tooltipMarkup, /model-c/)
  })
})
