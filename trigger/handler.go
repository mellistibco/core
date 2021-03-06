package trigger

import (
	"context"
	"errors"
	"fmt"

	"github.com/project-flogo/core/data/coerce"

	"github.com/project-flogo/core/action"
	"github.com/project-flogo/core/data"
	"github.com/project-flogo/core/data/expression"
	"github.com/project-flogo/core/data/mapper"
)

type Handler interface {
	Name() string
	Settings() map[string]interface{}
	Handle(ctx context.Context, triggerData interface{}) (map[string]interface{}, error)
}

type actImpl struct {
	act                action.Action
	condition          expression.Expr
	actionInputMapper  mapper.Mapper
	actionOutputMapper mapper.Mapper
}

type handlerImpl struct {
	runner action.Runner
	config *HandlerConfig
	acts   []actImpl
}

func (h *handlerImpl) Name() string {
	return h.config.Name
}

func (h *handlerImpl) Settings() map[string]interface{} {
	return h.config.Settings
}

func NewHandler(config *HandlerConfig, acts []action.Action, mf mapper.Factory, ef expression.Factory, runner action.Runner) (Handler, error) {

	handler := &handlerImpl{config: config, acts: make([]actImpl, len(acts)), runner: runner}

	var err error

	//todo we could filter inputs/outputs based on the metadata, maybe make this an option
	for i, act := range acts {
		handler.acts[i].act = act

		if config.Actions[i].If != "" {
			condition, err := ef.NewExpr(config.Actions[i].If)
			if err != nil {
				return nil, err
			}
			handler.acts[i].condition = condition
		}

		if len(config.Actions[i].Input) != 0 {
			handler.acts[i].actionInputMapper, err = mf.NewMapper(config.Actions[i].Input)
			if err != nil {
				return nil, err
			}
		}

		if len(config.Actions[i].Output) != 0 {
			handler.acts[i].actionOutputMapper, err = mf.NewMapper(config.Actions[i].Output)
			if err != nil {
				return nil, err
			}
		}
	}

	return handler, nil
}

func (h *handlerImpl) GetSetting(setting string) (interface{}, bool) {

	if h.config == nil {
		return nil, false
	}

	val, exists := h.config.Settings[setting]

	if !exists {
		val, exists = h.config.parent.Settings[setting]
	}

	return val, exists
}

func (h *handlerImpl) Handle(ctx context.Context, triggerData interface{}) (map[string]interface{}, error) {

	var err error

	var triggerValues map[string]interface{}

	if triggerData == nil {

	} else if values, ok := triggerData.(map[string]interface{}); ok {
		triggerValues = values
	} else if value, ok := triggerData.(data.StructValue); ok {
		triggerValues = value.ToMap()
	} else {
		return nil, fmt.Errorf("unsupported trigger data: %v", triggerData)
	}

	var act actImpl
	scope := data.NewSimpleScope(triggerValues, nil)
	for _, v := range h.acts {
		if v.condition == nil {
			act = v
			break
		}
		val, err := v.condition.Eval(scope)
		if err != nil {
			return nil, err
		}
		if val == nil {
			return nil, errors.New("expression has nil result")
		}
		condition, ok := val.(bool)
		if !ok {
			return nil, errors.New("expression has a non-bool result")
		}
		if condition {
			act = v
			break
		}
	}

	if act.act == nil {
		return nil, errors.New("no action to execute")
	}

	var inputMap map[string]interface{}

	if act.actionInputMapper != nil {
		inScope := data.NewSimpleScope(triggerValues, nil)

		inputMap, err = act.actionInputMapper.Apply(inScope)
		if err != nil {
			return nil, err
		}
	} else {
		inputMap = triggerValues
	}

	if ioMd := act.act.IOMetadata(); ioMd != nil {
		for name, tv := range ioMd.Input {
			if val, ok := inputMap[name]; ok {
				inputMap[name], err = coerce.ToType(val, tv.Type())
				if err != nil {
					return nil, err
				}
			}
		}
	}

	newCtx := NewHandlerContext(ctx, h.config)
	results, err := h.runner.RunAction(newCtx, act.act, inputMap)
	if err != nil {
		return nil, err
	}

	if act.actionOutputMapper != nil {
		outScope := data.NewSimpleScope(results, nil)
		retValue, err := act.actionOutputMapper.Apply(outScope)

		return retValue, err
	} else {
		return results, nil
	}
}

func (h *handlerImpl) String() string {
	return fmt.Sprintf("Handler")
}
